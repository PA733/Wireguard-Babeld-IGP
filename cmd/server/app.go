package main

import (
	"fmt"
	"net/http"

	"mesh-backend/internal/api"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/logger"
)

type App struct {
	server *http.Server
	logger *logger.Logger
}

func NewApp(handler *api.MixedHandler, cfg *config.Config, logger *logger.Logger) *App {
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: handler,
	}

	return &App{
		server: server,
		logger: logger,
	}
}

func (a *App) Run() error {
	log := a.logger.GetLogger("app")
	log.Info().
		Str("address", a.server.Addr).
		Msg("Starting server")

	return a.server.ListenAndServe()
}
