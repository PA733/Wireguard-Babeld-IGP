package types

import (
	"context"
	"mesh-backend/internal/models"
)

// Store 定义了存储层接口
type Store interface {
	// Node operations
	CreateNode(ctx context.Context, node *models.Node) error
	GetNode(ctx context.Context, id int) (*models.Node, error)
	ListNodes(ctx context.Context) ([]*models.Node, error)
	UpdateNode(ctx context.Context, node *models.Node) error
	DeleteNode(ctx context.Context, id int) error

	// Task operations
	CreateTask(ctx context.Context, task *models.Task) error
	GetTask(ctx context.Context, id string) (*models.Task, error)
	ListTasks(ctx context.Context) ([]*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
	DeleteTask(ctx context.Context, id string) error

	// Close releases any resources held by the store
	Close() error
}

// Config 存储配置
type Config struct {
	Type     string         `yaml:"type"`
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}
