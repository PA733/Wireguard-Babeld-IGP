package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "mesh-backend/api/proto/task"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/server/middleware"
	"mesh-backend/pkg/store"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TaskService 实现任务管理服务
type TaskService struct {
	pb.UnimplementedTaskServiceServer

	config *config.ServerConfig
	logger zerolog.Logger
	store  store.Store

	// 节点管理
	nodes    map[int32]*nodeState
	nodeMu   sync.RWMutex
	nodeAuth *middleware.NodeAuthenticator

	// 任务管理
	tasks    map[string]*types.Task
	tasksMu  sync.RWMutex
	taskChan chan *types.Task
}

// nodeState 记录节点状态
type nodeState struct {
	token      string
	lastSeen   time.Time
	stream     pb.TaskService_SubscribeTasksServer
	streamLock sync.Mutex
}

// NewTaskService 创建任务服务实例
func NewTaskService(cfg *config.ServerConfig, logger zerolog.Logger, store store.Store, nodeAuth *middleware.NodeAuthenticator) *TaskService {
	return &TaskService{
		config:   cfg,
		logger:   logger.With().Str("service", "task").Logger(),
		store:    store,
		nodes:    make(map[int32]*nodeState),
		tasks:    make(map[string]*types.Task),
		taskChan: make(chan *types.Task, 100),
		nodeAuth: nodeAuth,
	}
}

// RegisterGRPC 注册gRPC服务
func (s *TaskService) RegisterGRPC(server *grpc.Server) {
	pb.RegisterTaskServiceServer(server, s)
}

// Register 实现节点注册
func (s *TaskService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// 验证节点身份
	if !s.nodeAuth.ValidateToken(int(req.NodeId), req.Token) {
		return &pb.RegisterResponse{
			Success: false,
			Message: "Invalid credentials",
		}, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	// 更新节点状态
	s.nodeMu.Lock()
	s.nodes[req.NodeId] = &nodeState{
		token:    req.Token,
		lastSeen: time.Now(),
	}
	s.nodeMu.Unlock()

	return &pb.RegisterResponse{
		Success: true,
		Message: "Registration successful",
	}, nil
}

// SubscribeTasks 实现任务订阅
func (s *TaskService) SubscribeTasks(req *pb.SubscribeRequest, stream pb.TaskService_SubscribeTasksServer) error {
	// 验证节点身份
	if !s.nodeAuth.ValidateToken(int(req.NodeId), req.Token) {
		return status.Error(codes.Unauthenticated, "invalid credentials")
	}

	// 获取节点状态
	s.nodeMu.Lock()
	node, exists := s.nodes[req.NodeId]
	if !exists {
		s.nodeMu.Unlock()
		return status.Error(codes.NotFound, "node not registered")
	}

	// 更新流和最后活动时间
	node.streamLock.Lock()
	node.stream = stream
	node.lastSeen = time.Now()
	node.streamLock.Unlock()
	s.nodeMu.Unlock()

	// 保持连接直到客户端断开或上下文取消
	<-stream.Context().Done()

	// 清理节点状态
	s.nodeMu.Lock()
	if node, exists := s.nodes[req.NodeId]; exists {
		node.streamLock.Lock()
		node.stream = nil
		node.streamLock.Unlock()
	}
	s.nodeMu.Unlock()

	return nil
}

// UpdateTaskStatus 实现任务状态更新
func (s *TaskService) UpdateTaskStatus(ctx context.Context, req *pb.UpdateTaskStatusRequest) (*pb.UpdateTaskStatusResponse, error) {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	task, exists := s.tasks[req.TaskId]
	if !exists {
		return nil, status.Error(codes.NotFound, "task not found")
	}

	// 更新任务状态
	task.Status = types.TaskStatus(req.Status)
	if req.Error != "" {
		task.Message = req.Error
		task.Status = types.TaskStatusFailed
	}
	now := time.Now()
	task.CompletedAt = &now

	err := s.store.UpdateTask(task)
	if err != nil {
		return &pb.UpdateTaskStatusResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update task: %s", err),
		}, status.Error(codes.Internal, "failed to update task")
	}

	return &pb.UpdateTaskStatusResponse{
		Success: true,
		Message: "Task status updated",
	}, nil
}

// CreateTask 创建新任务
func (s *TaskService) CreateTask(taskType types.TaskType, nodeID int) (*types.Task, error) {
	task := &types.Task{
		ID:     generateTaskID(taskType),
		Type:   taskType,
		NodeID: nodeID,
		Status: types.TaskStatusPending,
	}

	s.tasksMu.Lock()
	s.tasks[task.ID] = task
	s.tasksMu.Unlock()

	// 发送任务到通道
	s.taskChan <- task

	// 保存任务到存储
	if err := s.store.CreateTask(task); err != nil {
		return nil, fmt.Errorf("saving task: %w", err)
	}

	return task, nil
}

// BroadcastTask 广播任务到所有节点
func (s *TaskService) BroadcastTask(task *types.Task) error {
	s.nodeMu.RLock()
	defer s.nodeMu.RUnlock()

	// 创建gRPC任务消息
	pbTask := &pb.Task{
		Id:   task.ID,
		Type: string(task.Type),
	}

	// 广播到所有节点
	for nodeID, node := range s.nodes {
		node.streamLock.Lock()
		if node.stream != nil {
			if err := node.stream.Send(pbTask); err != nil {
				s.logger.Error().
					Err(err).
					Int32("node_id", nodeID).
					Str("task_id", task.ID).
					Msg("Failed to send task to node")
			}
		}
		node.streamLock.Unlock()
	}

	return nil
}

// generateTaskID 生成任务ID
func generateTaskID(taskType types.TaskType) string {
	return fmt.Sprintf("%s_%d", string(taskType), time.Now().UnixNano())
}

// PushTask 推送任务
func (s *TaskService) PushTask(task *types.Task) error {
	now := time.Now()
	task.StartedAt = &now
	s.store.UpdateTask(task)

	// 查找节点状态
	s.nodeMu.RLock()
	node, exists := s.nodes[int32(task.NodeID)]
	s.nodeMu.RUnlock()

	if !exists {
		return fmt.Errorf("node %d not found", int32(task.NodeID))
	}

	// 推送任务到节点
	node.streamLock.Lock()
	defer node.streamLock.Unlock()

	if node.stream == nil {
		return fmt.Errorf("node %d stream not available", int32(task.NodeID))
	}

	// 转换为 protobuf 任务
	pbTask := &pb.Task{
		Id:   task.ID,
		Type: string(task.Type),
	}

	// 发送任务
	if err := node.stream.Send(pbTask); err != nil {
		return fmt.Errorf("sending task: %w", err)
	}

	s.logger.Info().
		Str("task_id", task.ID).
		Str("type", string(task.Type)).
		Int32("node_id", int32(task.NodeID)).
		Msg("Task pushed to node")

	return nil
}
