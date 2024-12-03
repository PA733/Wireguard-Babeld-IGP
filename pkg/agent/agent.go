package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Agent 代表一个网络节点代理
type Agent struct {
	config *config.AgentConfig
	logger zerolog.Logger

	// gRPC连接
	conn   *grpc.ClientConn
	client TaskServiceClient

	// 任务处理
	handlers []types.TaskHandler
	taskCh   chan *types.Task

	// 状态
	status     string
	statusLock sync.RWMutex

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
		taskCh:   make(chan *types.Task, 100),
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
	conn, err := grpc.Dial(
		a.config.Server.GRPCAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}

	a.conn = conn
	a.client = NewTaskServiceClient(conn)
	return nil
}

// register 注册节点
func (a *Agent) register() error {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	resp, err := a.client.Register(ctx, &RegisterRequest{
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
	stream, err := a.client.SubscribeTasks(a.ctx, &SubscribeRequest{
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
func (a *Agent) handleTaskStream(stream TaskService_SubscribeTasksClient) {
	for {
		task, err := stream.Recv()
		if err != nil {
			a.logger.Error().Err(err).Msg("Task stream error")
			// 尝试重新连接
			time.Sleep(5 * time.Second)
			if err := a.reconnect(); err != nil {
				a.logger.Error().Err(err).Msg("Failed to reconnect")
				continue
			}
			return
		}

		a.taskCh <- &types.Task{
			ID:     task.Id,
			Type:   types.TaskType(task.Type),
			Params: task.Params,
		}
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
func (a *Agent) processTask(task *types.Task) {
	logger := a.logger.With().
		Str("task_id", task.ID).
		Str("task_type", string(task.Type)).
		Logger()

	// 更新任务状态为运行中
	now := time.Now()
	task.Status = types.TaskStatusRunning
	task.StartedAt = &now

	// 查找合适的处理器
	var handler types.TaskHandler
	for _, h := range a.handlers {
		if h.CanHandle(task.Type) {
			handler = h
			break
		}
	}

	if handler == nil {
		logger.Error().Msg("No handler found for task type")
		a.updateTaskStatus(task, &types.TaskResult{
			Status:    types.TaskStatusFailed,
			Error:     "no handler found",
			Timestamp: time.Now(),
		})
		return
	}

	// 执行任务
	result, err := handler.Handle(task)
	if err != nil {
		logger.Error().Err(err).Msg("Task execution failed")
		if result == nil {
			result = &types.TaskResult{
				Status:    types.TaskStatusFailed,
				Error:     err.Error(),
				Timestamp: time.Now(),
			}
		}
	}

	// 更新任务状态
	a.updateTaskStatus(task, result)
}

// updateTaskStatus 更新任务状态
func (a *Agent) updateTaskStatus(task *types.Task, result *types.TaskResult) {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	_, err := a.client.UpdateTaskStatus(ctx, &UpdateTaskStatusRequest{
		TaskId: task.ID,
		Status: string(result.Status),
		Error:  result.Error,
	})

	if err != nil {
		a.logger.Error().
			Err(err).
			Str("task_id", task.ID).
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

	_, err := a.client.Heartbeat(ctx, &HeartbeatRequest{
		NodeId: int32(a.config.NodeID),
		Token:  a.config.Token,
	})
	return err
}
