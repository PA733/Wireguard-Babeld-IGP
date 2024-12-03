package main

import (
	"mesh-backend/internal/store/types"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/logger"

	"google.golang.org/grpc"
)

// LogFilePath 是日志文件路径的类型包装器
type LogFilePath string

func provideGRPCServer() *grpc.Server {
	return grpc.NewServer()
}

func provideStoreConfig(cfg *config.Config) *types.Config {
	return &types.Config{
		Type:     cfg.Storage.Type,
		SQLite:   types.SQLiteConfig(cfg.Storage.SQLite),
		Postgres: types.PostgresConfig(cfg.Storage.Postgres),
	}
}

func provideLoggerConfig(cfg *config.Config) bool {
	return cfg.Log.Debug
}

func provideLogFilePath(cfg *config.Config) LogFilePath {
	return LogFilePath(cfg.Log.File)
}

func provideLogger(debug bool, logFile LogFilePath) *logger.Logger {
	logger := logger.NewLogger(debug)
	if logFile != "" {
		logger.SetLogOutput(string(logFile))
	}
	return logger
}
