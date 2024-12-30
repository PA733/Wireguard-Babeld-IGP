package store

import (
	"github.com/glebarez/sqlite"
)

// SQLiteStore SQLite存储实现
type SQLiteStore struct {
	*GormStore
}

// NewSQLiteStore 创建SQLite存储实例
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	store, err := NewGormStore(sqlite.Open(path))
	if err != nil {
		return nil, err
	}

	return &SQLiteStore{GormStore: store}, nil
}
