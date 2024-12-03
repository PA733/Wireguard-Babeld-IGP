package factory

import (
	"fmt"

	"mesh-backend/internal/store/memory"
	"mesh-backend/internal/store/sqlite"
	"mesh-backend/internal/store/types"
)

// NewStore 创建新的存储实例
func NewStore(cfg *types.Config) (types.Store, error) {
	switch cfg.Type {
	case "memory":
		return memory.NewStore(), nil
	case "sqlite":
		return sqlite.NewStore(types.SQLiteConfig{
			Path: cfg.SQLite.Path,
		})
	case "postgres":
		// TODO: 实现 PostgreSQL 存储
		return nil, fmt.Errorf("postgres storage not implemented yet")
	default:
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Type)
	}
}
