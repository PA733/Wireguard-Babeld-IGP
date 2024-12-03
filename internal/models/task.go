package models

import (
	"time"
)

type TaskType string
type TaskStatus string

const (
	TaskTypeUpdate TaskType = "update"

	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusComplete TaskStatus = "complete"
	TaskStatusFailed   TaskStatus = "failed"
)

type Task struct {
	ID        string     `json:"task_id"`
	NodeID    int        `json:"node_id"`
	Type      TaskType   `json:"type"`
	Status    TaskStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
