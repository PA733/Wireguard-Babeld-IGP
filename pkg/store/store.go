package store

import (
	"context"
	"fmt"
	"time"

	"mesh-backend/pkg/types"
)

// Store 定义存储接口
type Store interface {
	// Node operations
	CreateNode(ctx context.Context, node *types.NodeConfig) error
	GetNode(ctx context.Context, nodeID int) (*types.NodeConfig, error)
	UpdateNode(ctx context.Context, nodeID int, node *types.NodeConfig) error
	DeleteNode(ctx context.Context, nodeID int) error
	ListNodes(ctx context.Context) ([]*types.NodeConfig, error)

	// Task operations
	CreateTask(ctx context.Context, task *types.Task) error
	GetTask(ctx context.Context, taskID string) (*types.Task, error)
	UpdateTask(ctx context.Context, taskID string, task *types.Task) error
	DeleteTask(ctx context.Context, taskID string) error
	ListTasks(ctx context.Context, filter TaskFilter) ([]*types.Task, error)

	// Status operations
	UpdateNodeStatus(ctx context.Context, nodeID int, status *types.NodeStatus) error
	GetNodeStatus(ctx context.Context, nodeID int) (*types.NodeStatus, error)
	ListNodeStatus(ctx context.Context) ([]*types.NodeStatus, error)

	// Maintenance
	Cleanup(ctx context.Context) error
	Close() error
}

// TaskFilter 定义任务过滤条件
type TaskFilter struct {
	NodeID    *int
	Status    *types.TaskStatus
	Type      *types.TaskType
	StartTime *int64
	EndTime   *int64
	Limit     int
	Offset    int
}

// Config 存储配置
type Config struct {
	Type     string       `json:"type"`     // 存储类型：sqlite, postgres
	SQLite   SQLiteConfig `json:"sqlite"`   // SQLite配置
	Postgres interface{}  `json:"postgres"` // PostgreSQL配置（暂未实现）
}

// SQLiteConfig SQLite配置
type SQLiteConfig struct {
	Path            string        `json:"path"`               // 数据库文件路径
	MaxOpenConns    int           `json:"max_open_conns"`     // 最大打开连接数
	MaxIdleConns    int           `json:"max_idle_conns"`     // 最大空闲连接数
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`  // 连接最大生命周期
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time"` // 连接最大空闲时间
}

// NewStore 创建存储实例
func NewStore(cfg *Config) (Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	switch cfg.Type {
	case "sqlite":
		return NewSQLiteStore(&cfg.SQLite)
	case "postgres":
		return nil, fmt.Errorf("postgres storage not implemented")
	default:
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Type)
	}
}
