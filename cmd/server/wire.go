//go:build wireinject
// +build wireinject

package main

import (
	"mesh-backend/internal/api"
	"mesh-backend/internal/api/handlers"
	"mesh-backend/internal/service"
	"mesh-backend/internal/store/factory"
	"mesh-backend/pkg/config"

	"github.com/google/wire"
)

func InitializeApp(configPath string) (*App, error) {
	wire.Build(
		// 基础设施
		config.LoadConfig,
		provideLoggerConfig,
		provideLogFilePath,
		provideLogger,
		provideGRPCServer,

		// Store
		provideStoreConfig,
		factory.NewStore,

		// Services
		service.NewTaskService,
		service.NewNodeService,
		service.NewConfigService,
		service.NewStatusService,

		// Handlers
		handlers.NewNodeHandler,
		handlers.NewTaskHandler,
		handlers.NewConfigHandler,
		handlers.NewStatusHandler,

		// Router & Server
		api.NewRouter,
		api.NewMixedHandler,
		NewApp,
	)
	return nil, nil
}
