package types

import "time"

// TaskType 定义任务类型
type TaskType string

const (
	TaskTypeUpdate TaskType = "update" // 更新配置
	TaskTypeStatus TaskType = "status" // 状态报告
)

// TaskStatus 定义任务状态
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"  // 等待执行
	TaskStatusRunning  TaskStatus = "running"  // 正在执行
	TaskStatusSuccess  TaskStatus = "success"  // 执行成功
	TaskStatusFailed   TaskStatus = "failed"   // 执行失败
	TaskStatusCanceled TaskStatus = "canceled" // 已取消
)

// Task 定义任务结构
type Task struct {
	ID          string     `gorm:"primarykey;size:36" json:"id"` // 任务ID
	Type        TaskType   `gorm:"size:50" json:"type"`          // 任务类型
	Status      TaskStatus `gorm:"size:50" json:"status"`        // 任务状态
	Params      string     `gorm:"type:text" json:"params"`      // 任务参数(JSON)
	CreatedAt   time.Time  `json:"created_at"`                   // 创建时间
	UpdatedAt   time.Time  `json:"updated_at"`                   // 更新时间
	StartedAt   *time.Time `json:"started_at"`                   // 开始时间
	CompletedAt *time.Time `json:"completed_at"`                 // 完成时间
}

// TaskResult 定义任务执行结果
type TaskResult struct {
	ID        int        `gorm:"primarykey" json:"-"`
	TaskID    string     `gorm:"size:36;index" json:"task_id"`
	Status    TaskStatus `gorm:"size:50" json:"status"`    // 执行状态
	Details   string     `gorm:"type:text" json:"details"` // 详细信息(JSON)
	Error     string     `gorm:"type:text" json:"error"`   // 错误信息
	Timestamp time.Time  `json:"timestamp"`                // 时间戳
}

// TaskHandler 定义任务处理器接口
type TaskHandler interface {
	// Handle 处理任务
	Handle(task *Task) (*TaskResult, error)
	// CanHandle 检查是否可以处理该类型的任务
	CanHandle(taskType TaskType) bool
}
