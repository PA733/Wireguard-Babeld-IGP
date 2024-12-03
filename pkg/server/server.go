package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/server/services"
	"mesh-backend/pkg/store"
)

// Server 服务器结构
type Server struct {
	config *config.ServerConfig
	logger zerolog.Logger
	store  store.Store

	// 服务实例
	nodeService   *services.NodeService
	configService *services.ConfigService
	taskService   *services.TaskService
	statusService *services.StatusService

	// 服务器实例
	grpcServer *grpc.Server
	httpServer *http.Server
	wg         sync.WaitGroup
}

// New 创建服务器实例
func New(cfg *config.ServerConfig, logger zerolog.Logger) (*Server, error) {
	// 创建存储实例
	store, err := store.NewStore(&store.Config{
		Type: cfg.Storage.Type,
		SQLite: store.SQLiteConfig{
			Path: cfg.Storage.SQLite.Path,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}

	// 创建服务实例
	taskService := services.NewTaskService(cfg, logger, store)
	nodeService := services.NewNodeService(cfg, logger, store, taskService)
	configService := services.NewConfigService(cfg, logger, nodeService, taskService)
	statusService := services.NewStatusService(cfg, logger, store)

	// 创建gRPC服务器
	var opts []grpc.ServerOption
	if cfg.Server.TLS.Enabled {
		creds, err := credentials.NewServerTLSFromFile(cfg.Server.TLS.Cert, cfg.Server.TLS.Key)
		if err != nil {
			return nil, fmt.Errorf("loading TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}
	grpcServer := grpc.NewServer(opts...)

	// 注册服务
	taskService.RegisterGRPC(grpcServer)
	reflection.Register(grpcServer)

	// 创建gRPC-Web包装器
	wrappedGrpc := grpcweb.WrapServer(grpcServer,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return true // 允许所有来源，生产环境应该限制
		}),
	)

	// 创建HTTP处理器
	mux := http.NewServeMux()

	// 注册HTTP路由
	mux.HandleFunc("/nodes", nodeService.HandleListNodes)
	mux.HandleFunc("/nodes/", nodeService.HandleGetNode)
	mux.HandleFunc("/config/", configService.HandleGetConfig)
	mux.HandleFunc("/status", statusService.HandleGetStatus)

	// 创建HTTP服务器
	httpServer := &http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if wrappedGrpc.IsGrpcWebRequest(r) {
				wrappedGrpc.ServeHTTP(w, r)
				return
			}
			mux.ServeHTTP(w, r)
		}),
	}

	return &Server{
		config:        cfg,
		logger:        logger.With().Str("component", "server").Logger(),
		store:         store,
		nodeService:   nodeService,
		configService: configService,
		taskService:   taskService,
		statusService: statusService,
		grpcServer:    grpcServer,
		httpServer:    httpServer,
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	// 初始化服务
	if err := s.configService.InitTemplates(); err != nil {
		return fmt.Errorf("initializing config templates: %w", err)
	}

	// 启动HTTP/gRPC服务器
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info().
			Str("address", s.httpServer.Addr).
			Msg("Starting HTTP/gRPC server")
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("HTTP/gRPC server error")
		}
	}()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	// 停止HTTP服务器
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		s.logger.Error().Err(err).Msg("Error shutting down HTTP server")
	}

	// 停止gRPC服务器
	s.grpcServer.GracefulStop()

	// 等待所有服务停止
	s.wg.Wait()

	// 关闭存储
	if err := s.store.Close(); err != nil {
		s.logger.Error().Err(err).Msg("Error closing store")
	}

	return nil
}
