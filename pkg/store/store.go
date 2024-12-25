package store

import (
	"fmt"

	"mesh-backend/pkg/types"
)

// Store 定义存储接口
type Store interface {
	// 节点相关
	CreateNode(node *types.NodeConfig) error
	GetNode(nodeID int) (*types.NodeConfig, error)
	UpdateNode(nodeID int, node *types.NodeConfig) error
	DeleteNode(nodeID int) error
	ListNodes() ([]*types.NodeConfig, error)
	GetOrCreateWireguardConnection(connection *types.WireguardConnection, basePort int) (*types.WireguardConnection, error)

	// 节点状态相关
	UpdateNodeStatus(nodeID int, status *types.NodeStatus) error
	GetNodeStatus(nodeID int) (*types.NodeStatus, error)
	ListNodeStatus() ([]*types.NodeStatus, error)

	// 任务相关
	SaveTask(task *types.Task) error
	GetTask(id string) (*types.Task, error)
	ListTasks(filter TaskFilter) ([]*types.Task, error)
	DeleteTask(id string) error
	CleanupTasks() error

	// 关闭存储
	Close() error
}

// Config 存储配置
type Config struct {
	Type     string         `yaml:"type"`     // 存储类型
	SQLite   SQLiteConfig   `yaml:"sqlite"`   // SQLite配置
	Postgres PostgresConfig `yaml:"postgres"` // Postgre配置
}

// SQLiteConfig SQLite配置
type SQLiteConfig struct {
	Path string `yaml:"path"` // 数据库文件路径
}

// PostgresConfig Postgre配置
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// NewStore 创建存储实例
func NewStore(cfg *Config) (Store, error) {
	switch cfg.Type {
	case "memory":
		return NewMemoryStore(), nil
	case "sqlite":
		return NewSQLiteStore(cfg.SQLite.Path)
	case "postgres":
		return NewPostgreStore(cfg.Postgres)
	default:
		return nil, fmt.Errorf("unsupported store type: %s", cfg.Type)
	}
}
