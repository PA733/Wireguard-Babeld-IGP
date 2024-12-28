package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pb "mesh-backend/api/proto/task"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Agent 代表一个网络节点代理
type Agent struct {
	config *config.AgentConfig
	logger zerolog.Logger

	// gRPC连接
	conn   *grpc.ClientConn
	client pb.TaskServiceClient

	// 任务处理
	handlers []types.TaskHandler
	taskCh   chan *pb.Task

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建新的Agent实例
func New(cfg *config.AgentConfig, logger zerolog.Logger) (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		config:   cfg,
		logger:   logger,
		taskCh:   make(chan *pb.Task, 100),
		ctx:      ctx,
		cancel:   cancel,
		handlers: make([]types.TaskHandler, 0),
	}, nil
}

// Start 启动Agent
func (a *Agent) Start() error {
	// 连接gRPC服务器
	if err := a.connect(); err != nil {
		return fmt.Errorf("connecting to server: %w", err)
	}

	// 注册节点
	if err := a.register(); err != nil {
		return fmt.Errorf("registering node: %w", err)
	}

	// 启动任务订阅
	if err := a.subscribeTasks(); err != nil {
		return fmt.Errorf("subscribing to tasks: %w", err)
	}

	// 启动任务处理
	go a.processTasksLoop()

	// 启动心跳
	go a.heartbeatLoop()

	return nil
}

// Stop 停止Agent
func (a *Agent) Stop() error {
	a.cancel()
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// RegisterHandler 注册任务处理器
func (a *Agent) RegisterHandler(handler types.TaskHandler) {
	a.handlers = append(a.handlers, handler)
}

// connect 连接到gRPC服务器
func (a *Agent) connect() error {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	// 设置 gRPC 连接选项
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // 每10秒发送一次keepalive ping
			Timeout:             3 * time.Second,  // 3秒内没有响应则认为连接断开
			PermitWithoutStream: true,             // 允许在没有活动流的情况下发送keepalive
		}),
		grpc.WithDefaultServiceConfig(`{
			"methodConfig": [{
				"name": [{"service": "task.TaskService"}],
				"retryPolicy": {
					"MaxAttempts": 5,
					"InitialBackoff": "0.1s",
					"MaxBackoff": "5s",
					"BackoffMultiplier": 2.0,
					"RetryableStatusCodes": ["UNAVAILABLE"]
				}
			}]
		}`),
	}

	// 连接服务器
	conn, err := grpc.DialContext(
		ctx,
		a.config.Server.GRPCAddress,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("connecting to server: %w", err)
	}

	a.conn = conn
	a.client = pb.NewTaskServiceClient(conn)
	return nil
}

// register 注册节点
func (a *Agent) register() error {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	resp, err := a.client.Register(ctx, &pb.RegisterRequest{
		NodeId: int32(a.config.NodeID),
		Token:  a.config.Token,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	return nil
}

// subscribeTasks 订阅任务
func (a *Agent) subscribeTasks() error {
	stream, err := a.client.SubscribeTasks(a.ctx, &pb.SubscribeRequest{
		NodeId: int32(a.config.NodeID),
		Token:  a.config.Token,
	})
	if err != nil {
		return err
	}

	go a.handleTaskStream(stream)
	return nil
}

// handleTaskStream 处理任务流
func (a *Agent) handleTaskStream(stream pb.TaskService_SubscribeTasksClient) {
	for {
		task, err := stream.Recv()
		if err != nil {
			a.logger.Error().Err(err).Msg("Task stream error")

			// 检查是否是节点未注册错误
			if strings.Contains(err.Error(), "node not registered") {
				a.logger.Info().Msg("Node not registered, attempting to re-register")
				// 尝试重新注册
				if err := a.register(); err != nil {
					a.logger.Error().Err(err).Msg("Failed to re-register")
					time.Sleep(5 * time.Second)
					continue
				}
				// 重新订阅任务
				if err := a.subscribeTasks(); err != nil {
					a.logger.Error().Err(err).Msg("Failed to re-subscribe tasks")
					time.Sleep(5 * time.Second)
					continue
				}
				return
			}

			// 其他错误，尝试重新连接
			time.Sleep(5 * time.Second)
			if err := a.reconnect(); err != nil {
				a.logger.Error().Err(err).Msg("Failed to reconnect")
				continue
			}
			return
		}

		a.taskCh <- task
	}
}

// processTasksLoop 处理任务循环
func (a *Agent) processTasksLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case task := <-a.taskCh:
			go a.processTask(task)
		}
	}
}

// processTask 处理单个任务
func (a *Agent) processTask(task *pb.Task) {
	a.logger.Info().
		Str("task_id", task.Id).
		Str("type", task.Type).
		Msg("Processing task")

	var err error
	switch task.Type {
	case string(types.TaskTypeUpdate):
		err = a.handleConfigUpdate(task)
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	if err != nil {
		a.logger.Error().Err(err).
			Str("task_id", task.Id).
			Msg("Failed to process task")
	}
}

// handleConfigUpdate 处理配置更新任务
func (a *Agent) handleConfigUpdate(task *pb.Task) error {
	// 获取最新配置
	url := fmt.Sprintf("%s/api/agent/config/%d", a.config.Server.Address, a.config.NodeID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("fetching config: %w", err)
	}

	auth := fmt.Sprintf("%d:%s", a.config.NodeID, a.config.Token)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Add("Authorization", "Basic "+encodedAuth)
	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败:%s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var config types.NodeConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return fmt.Errorf("decoding config: %w", err)
	}

	// 更新 WireGuard 配置
	var configs map[string]string
	err = json.Unmarshal([]byte(config.WireGuard), &configs)
	if err != nil {
		log.Fatal(err)
	}
	if err := a.updateWireGuardConfig(configs); err != nil {
		return fmt.Errorf("updating wireguard config: %w", err)
	}

	// 更新 Babeld 配置
	if err := a.updateBabeldConfig(config.Babel); err != nil {
		return fmt.Errorf("updating babeld config: %w", err)
	}

	a.updateTaskStatus(task, &types.TaskResult{
		Status: types.TaskStatusSuccess,
	})
	a.logger.Info().Msg("Configuration updated successfully")
	return nil
}

// updateWireGuardConfig 更新 WireGuard 配置
func (a *Agent) updateWireGuardConfig(configs map[string]string) error {
	for peerName, config := range configs {
		configPath := filepath.Join(a.config.WireGuard.ConfigPath, fmt.Sprintf("%s%s.conf", a.config.WireGuard.Prefix, peerName))
		if !a.config.Runtime.DryRun {
			if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
				return fmt.Errorf("writing wireguard config: %w", err)
			}
		} else {
			a.logger.Info().Str("DryRun", "wireguard_config").Str("path", configPath).Msg("Would run: " + config)
		}

		// 重启 WireGuard 接口
		if err := a.RestartWireGuard(fmt.Sprintf("%s%s", a.config.WireGuard.Prefix, peerName)); err != nil {
			return fmt.Errorf("restarting wireguard: %w", err)
		}
	}
	return nil
}

// RestartWireGuard 重启 WireGuard
func (a *Agent) RestartWireGuard(interfaceName string) error {
	cmd := exec.Command("systemctl", "restart", "wg-quick@"+interfaceName)
	if !a.config.Runtime.DryRun {
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restarting wireguard: %w", err)
		}
	} else {
		a.logger.Info().Str("DryRun", "wireguard_interface").Msg("Would run: " + cmd.String())
	}
	return nil
}

// updateBabeldConfig 更新 Babeld 配置
func (a *Agent) updateBabeldConfig(config string) error {
	config = strings.ReplaceAll(config, "{WGPrefix}", a.config.WireGuard.Prefix)
	if !a.config.Runtime.DryRun {
		if err := os.WriteFile(a.config.Babel.ConfigPath, []byte(config), 0644); err != nil {
			return fmt.Errorf("writing babeld config: %w", err)
		}
	} else {
		a.logger.Info().Str("DryRun", "babeld_config").Msg("Would run: " + config)
	}

	// 重启 Babeld 进程
	if err := a.RestartBabeld(); err != nil {
		return fmt.Errorf("restarting babeld: %w", err)
	}
	return nil
}

func (a *Agent) RestartBabeld() error {
	cmd := exec.Command("systemctl", "restart", "babeld")
	if !a.config.Runtime.DryRun {
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restarting babeld: %w", err)
		}
	} else {
		a.logger.Info().Str("DryRun", "babeld").Msg("Would run: " + cmd.String())
	}
	return nil
}

// updateTaskStatus 更新任务状态
func (a *Agent) updateTaskStatus(task *pb.Task, result *types.TaskResult) {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	req := &pb.UpdateTaskStatusRequest{
		TaskId: task.Id,
		Status: string(result.Status),
		Error:  result.Error,
	}

	_, err := a.client.UpdateTaskStatus(ctx, req)
	if err != nil {
		a.logger.Error().
			Err(err).
			Str("task_id", task.Id).
			Msg("Failed to update task status")
	}
}

// reconnect 重新连接到服务器
func (a *Agent) reconnect() error {
	if a.conn != nil {
		a.conn.Close()
	}

	if err := a.connect(); err != nil {
		return err
	}

	if err := a.register(); err != nil {
		return err
	}

	return a.subscribeTasks()
}

// heartbeatLoop 心跳循环
func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.sendHeartbeat(); err != nil {
				a.logger.Error().Err(err).Msg("Heartbeat failed")
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat() error {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	req := &pb.HeartbeatRequest{
		NodeId: int32(a.config.NodeID),
		Token:  a.config.Token,
		Status: map[string]string{
			"status": "running",
			"time":   time.Now().Format(time.RFC3339),
		},
	}

	_, err := a.client.Heartbeat(ctx, req)
	return err
}
