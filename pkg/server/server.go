package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
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
	listener   net.Listener
	mux        cmux.CMux
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

	// 创建共享的 NodeAuthenticator
	nodeAuth := services.NewNodeAuthenticator(logger)

	// 创建服务实例
	taskService := services.NewTaskService(cfg, logger, store, nodeAuth)
	nodeService := services.NewNodeService(cfg, logger, store, taskService, nodeAuth)
	configService, err := services.NewConfigService(cfg, nodeService, nodeAuth, logger, taskService)
	if err != nil {
		return nil, fmt.Errorf("creating config service: %w", err)
	}
	statusService := services.NewStatusService(cfg, logger, store)

	// 创建基础TCP监听器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("creating listener: %w", err)
	}

	// 创建多路复用器
	mux := cmux.New(listener)

	// 创建gRPC服务器
	var opts []grpc.ServerOption
	if cfg.Server.TLS.Enabled {
		creds, err := credentials.NewServerTLSFromFile(cfg.Server.TLS.Cert, cfg.Server.TLS.Key)
		if err != nil {
			return nil, fmt.Errorf("loading TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// 添加服务器选项
	opts = append(opts,
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Second,
			MaxConnectionAge:      30 * time.Second,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Second,
			Timeout:               1 * time.Second,
		}),
	)

	grpcServer := grpc.NewServer(opts...)

	// 注册服务
	taskService.RegisterGRPC(grpcServer)
	reflection.Register(grpcServer)

	// 创建HTTP处理器
	httpMux := http.NewServeMux()

	// 注册HTTP路由
	httpMux.HandleFunc("/nodes", nodeService.HandleListNodes)
	httpMux.HandleFunc("/nodes/", nodeService.HandleGetNode)
	httpMux.HandleFunc("/nodes/config/", nodeService.HandleTriggerConfigUpdate)
	httpMux.HandleFunc("/config/", configService.HandleGetConfig)
	httpMux.HandleFunc("/status", statusService.HandleGetStatus)

	// 创建HTTP服务器
	httpServer := &http.Server{
		Handler: httpMux,
	}

	return &Server{
		config:        cfg,
		logger:        logger.With().Str("component", "server").Logger(),
		store:         store,
		nodeService:   nodeService,
		configService: configService,
		taskService:   taskService,
		statusService: statusService,
		listener:      listener,
		mux:           mux,
		grpcServer:    grpcServer,
		httpServer:    httpServer,
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	// 设置 gRPC 匹配器
	grpcL := s.mux.MatchWithWriters(
		cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"),
	)

	// 设置 HTTP 匹配器
	httpL := s.mux.Match(cmux.HTTP1Fast())

	// 启动 gRPC 服务器
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.grpcServer.Serve(grpcL); err != nil {
			s.logger.Error().Err(err).Msg("gRPC server error")
		}
	}()

	// 启动 HTTP 服务器
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.Serve(httpL); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// 启动 cmux
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.mux.Serve(); err != nil {
			s.logger.Error().Err(err).Msg("cmux server error")
		}
	}()

	s.logger.Info().
		Str("address", fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)).
		Bool("tls", s.config.Server.TLS.Enabled).
		Msg("Server started")

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	// 优雅关闭 HTTP 服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error().Err(err).Msg("Error shutting down HTTP server")
	}

	// 优雅关闭 gRPC 服务器
	s.grpcServer.GracefulStop()

	// 关闭监听器
	if err := s.listener.Close(); err != nil {
		s.logger.Error().Err(err).Msg("Error closing listener")
	}

	// 等待所有服务停止
	s.wg.Wait()

	// 关闭存储
	if err := s.store.Close(); err != nil {
		s.logger.Error().Err(err).Msg("Error closing store")
	}

	s.logger.Info().Msg("Server stopped")
	return nil
}
