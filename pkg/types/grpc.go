package types

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// TaskServiceClient 定义任务服务客户端接口
type TaskServiceClient interface {
	// Register 注册节点
	Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error)
	// SubscribeTasks 订阅任务
	SubscribeTasks(ctx context.Context, req *SubscribeRequest) (TaskService_SubscribeTasksClient, error)
	// UpdateTaskStatus 更新任务状态
	UpdateTaskStatus(ctx context.Context, req *UpdateTaskStatusRequest) (*UpdateTaskStatusResponse, error)
	// Heartbeat 发送心跳
	Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error)
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	NodeID int32  `json:"node_id"`
	Token  string `json:"token"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SubscribeRequest 订阅请求
type SubscribeRequest struct {
	NodeID int32  `json:"node_id"`
	Token  string `json:"token"`
}

// UpdateTaskStatusRequest 更新任务状态请求
type UpdateTaskStatusRequest struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// UpdateTaskStatusResponse 更新任务状态响应
type UpdateTaskStatusResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	NodeID int32     `json:"node_id"`
	Token  string    `json:"token"`
	Time   time.Time `json:"time"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// TaskService_SubscribeTasksClient 定义任务订阅流客户端接口
type TaskService_SubscribeTasksClient interface {
	Recv() (*Task, error)
	grpc.ClientStream
}

// taskServiceClient 实现TaskServiceClient接口
type taskServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewTaskServiceClient 创建任务服务客户端
func NewTaskServiceClient(cc grpc.ClientConnInterface) TaskServiceClient {
	return &taskServiceClient{cc}
}

// Register 实现注册方法
func (c *taskServiceClient) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	var resp RegisterResponse
	err := c.cc.Invoke(ctx, "/task.TaskService/Register", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// SubscribeTasks 实现任务订阅方法
func (c *taskServiceClient) SubscribeTasks(ctx context.Context, req *SubscribeRequest) (TaskService_SubscribeTasksClient, error) {
	stream, err := c.cc.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "SubscribeTasks",
		ServerStreams: true,
		ClientStreams: false,
	}, "/task.TaskService/SubscribeTasks")
	if err != nil {
		return nil, err
	}
	if err := stream.SendMsg(req); err != nil {
		return nil, err
	}
	return &taskServiceSubscribeTasksClient{stream}, nil
}

// UpdateTaskStatus 实现更新任务状态方法
func (c *taskServiceClient) UpdateTaskStatus(ctx context.Context, req *UpdateTaskStatusRequest) (*UpdateTaskStatusResponse, error) {
	var resp UpdateTaskStatusResponse
	err := c.cc.Invoke(ctx, "/task.TaskService/UpdateTaskStatus", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Heartbeat 实现心跳方法
func (c *taskServiceClient) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	var resp HeartbeatResponse
	err := c.cc.Invoke(ctx, "/task.TaskService/Heartbeat", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// taskServiceSubscribeTasksClient 实现TaskService_SubscribeTasksClient接口
type taskServiceSubscribeTasksClient struct {
	grpc.ClientStream
}

// Recv 实现接收任务方法
func (x *taskServiceSubscribeTasksClient) Recv() (*Task, error) {
	var task Task
	if err := x.ClientStream.RecvMsg(&task); err != nil {
		return nil, err
	}
	return &task, nil
}
