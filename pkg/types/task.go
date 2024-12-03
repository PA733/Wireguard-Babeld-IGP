package types

import "time"

// TaskType 定义任务类型
type TaskType string

const (
	TaskTypeUpdate TaskType = "update" // 配置更新任务
	TaskTypeCheck  TaskType = "check"  // 状态检查任务
)

// TaskStatus 定义任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 等待执行
	TaskStatusRunning   TaskStatus = "running"   // 正在执行
	TaskStatusSuccess   TaskStatus = "success"   // 执行成功
	TaskStatusFailed    TaskStatus = "failed"    // 执行失败
	TaskStatusCancelled TaskStatus = "cancelled" // 已取消
)

// Task 定义任务结构
type Task struct {
	ID          string                 `json:"id"`           // 任务ID
	Type        TaskType               `json:"type"`         // 任务类型
	Status      TaskStatus             `json:"status"`       // 任务状态
	Params      map[string]interface{} `json:"params"`       // 任务参数
	CreatedAt   time.Time              `json:"created_at"`   // 创建时间
	StartedAt   *time.Time             `json:"started_at"`   // 开始执行时间
	CompletedAt *time.Time             `json:"completed_at"` // 完成时间
	Error       string                 `json:"error"`        // 错误信息
}

// TaskResult 定义任务执行结果
type TaskResult struct {
	Status    TaskStatus `json:"status"`    // 任务状态
	Error     string     `json:"error"`     // 错误信息
	Details   string     `json:"details"`   // 详细信息
	Timestamp time.Time  `json:"timestamp"` // 时间戳
}

// TaskHandler 定义任务处理器接口
type TaskHandler interface {
	// Handle 处理任务
	Handle(task *Task) (*TaskResult, error)
	// CanHandle 检查是否可以处理该类型的任务
	CanHandle(taskType TaskType) bool
}
