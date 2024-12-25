package store

import (
	"fmt"

	"gorm.io/driver/postgres"
)

// PostgreStore PostgreSQL存储实现
type PostgreStore struct {
	*GormStore
}

// NewPostgreStore 创建PostgreSQL存储实例
func NewPostgreStore(config PostgresConfig) (*PostgreStore, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode)

	store, err := NewGormStore(postgres.Open(dsn))
	if err != nil {
		return nil, err
	}

	return &PostgreStore{GormStore: store}, nil
}
