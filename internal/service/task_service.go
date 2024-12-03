package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "mesh-backend/api/proto/task"
	"mesh-backend/internal/models"
	"mesh-backend/internal/store/types"
)

type TaskService struct {
	store   types.Store
	streams map[int][]pb.TaskService_SubscribeTasksServer
	mu      sync.RWMutex
	pb.UnimplementedTaskServiceServer
}

func NewTaskService(store types.Store) *TaskService {
	return &TaskService{
		store:   store,
		streams: make(map[int][]pb.TaskService_SubscribeTasksServer),
	}
}

// Register 实现节点注册
func (s *TaskService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	node, err := s.store.GetNode(ctx, int(req.NodeId))
	if err != nil {
		return &pb.RegisterResponse{
			Success: false,
			Message: "Node not found",
		}, nil
	}

	if node.Token != req.Token {
		return &pb.RegisterResponse{
			Success: false,
			Message: "Invalid token",
		}, nil
	}

	return &pb.RegisterResponse{
		Success: true,
		Message: "Registered successfully",
	}, nil
}

// SubscribeTasks 实现任务订阅
func (s *TaskService) SubscribeTasks(req *pb.RegisterRequest, stream pb.TaskService_SubscribeTasksServer) error {
	// 验证节点
	resp, err := s.Register(stream.Context(), req)
	if err != nil || !resp.Success {
		return fmt.Errorf("registration failed: %v", err)
	}

	// 添加流到节点的订阅列表
	s.mu.Lock()
	nodeID := int(req.NodeId)
	s.streams[nodeID] = append(s.streams[nodeID], stream)
	s.mu.Unlock()

	// 等待连接关闭
	<-stream.Context().Done()

	// 清理流
	s.mu.Lock()
	streams := s.streams[nodeID]
	for i, str := range streams {
		if str == stream {
			s.streams[nodeID] = append(streams[:i], streams[i+1:]...)
			break
		}
	}
	s.mu.Unlock()

	return nil
}

// UpdateTaskStatus 实现任务状态更新
func (s *TaskService) UpdateTaskStatus(ctx context.Context, status *pb.TaskStatus) (*pb.TaskStatusResponse, error) {
	task, err := s.store.GetTask(ctx, status.TaskId)
	if err != nil {
		return &pb.TaskStatusResponse{
			Success: false,
			Message: "Task not found",
		}, nil
	}

	task.Status = models.TaskStatus(status.Status)
	task.UpdatedAt = time.Unix(status.UpdatedAt, 0)

	if err := s.store.UpdateTask(ctx, task); err != nil {
		return &pb.TaskStatusResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update task: %v", err),
		}, nil
	}

	return &pb.TaskStatusResponse{
		Success: true,
		Message: "Status updated",
	}, nil
}

// CreateTask 创建新任务并推送到相关节点
func (s *TaskService) CreateTask(nodeID int, taskType models.TaskType) (*models.Task, error) {
	ctx := context.Background()

	task := &models.Task{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		NodeID:    nodeID,
		Type:      taskType,
		Status:    models.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.store.CreateTask(ctx, task); err != nil {
		return nil, err
	}

	s.pushTask(task)
	return task, nil
}

// pushTask 将任务推送到订阅的节点
func (s *TaskService) pushTask(task *models.Task) {
	s.mu.RLock()
	streams := s.streams[task.NodeID]
	s.mu.RUnlock()

	protoTask := &pb.Task{
		TaskId:    task.ID,
		NodeId:    int32(task.NodeID),
		Type:      string(task.Type),
		CreatedAt: task.CreatedAt.Unix(),
		Params:    make(map[string]string),
	}

	for _, stream := range streams {
		if err := stream.Send(protoTask); err != nil {
			// 处理发送错误，可能需要移除失效的流
			continue
		}
	}
}

func (s *TaskService) ListTasks() ([]*models.Task, error) {
	ctx := context.Background()
	return s.store.ListTasks(ctx)
}
