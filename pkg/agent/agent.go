package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "mesh-backend/api/proto/task"
	"mesh-backend/pkg/agent/handlers"
	"mesh-backend/pkg/config"

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
	taskHandler *handlers.TaskHandler

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建新的Agent实例
func New(cfg *config.AgentConfig, logger zerolog.Logger) (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		config: cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start 启动Agent
func (a *Agent) Start() error {
	// 连接gRPC服务器
	if err := a.connect(); err != nil {
		return fmt.Errorf("connecting to server: %w", err)
	}

	// 初始化任务处理器
	a.taskHandler = handlers.NewTaskHandler(a.config, a.logger, a.client, a.ctx)
	a.taskHandler.Start()

	// 注册节点
	if err := a.register(); err != nil {
		return fmt.Errorf("registering node: %w", err)
	}

	// 启动任务订阅
	if err := a.subscribeTasks(); err != nil {
		return fmt.Errorf("subscribing to tasks: %w", err)
	}

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

		a.taskHandler.EnqueueTask(task)
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
