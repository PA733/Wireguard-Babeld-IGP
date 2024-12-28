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
	"mesh-backend/pkg/server/middleware"
	"mesh-backend/pkg/server/services"
	"mesh-backend/pkg/store"

	"github.com/gin-gonic/gin"
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
	userService   *services.UserService

	// 服务器实例
	listener   net.Listener
	mux        cmux.CMux
	grpcServer *grpc.Server
	httpServer *gin.Engine
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
		Postgres: cfg.Storage.Postgres,
	})
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}

	// 创建认证中间件
	jwtAuth := middleware.NewJWTAuthenticator(logger, []byte(cfg.Server.JWT.SecretKey))
	nodeAuth := middleware.NewNodeAuthenticator(logger, store)

	// 创建服务实例
	taskService := services.NewTaskService(cfg, logger, store, nodeAuth)
	nodeService := services.NewNodeService(cfg, logger, store, taskService)
	configService, err := services.NewConfigService(cfg, nodeService, logger, taskService)
	if err != nil {
		return nil, fmt.Errorf("creating config service: %w", err)
	}
	statusService := services.NewStatusService(cfg, logger, store)
	userService := services.NewUserService(cfg, logger, store, *jwtAuth)

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

	// 创建 Gin 引擎
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		{
			userService.RegisterRoutes(auth)
		}

		// 创建需要JWT认证的路由组
		dashboard := api.Group("/dashboard")
		dashboard.Use(jwtAuth.JWTAuth())
		{
			nodeService.RegisterRoutes(dashboard)
			statusService.RegisterRoutes(dashboard)
		}

		agent := router.Group("/agent")
		agent.Use(nodeAuth.NodeAuth())
		{
			configService.RegisterRoutes(agent)
		}
	}

	return &Server{
		config:        cfg,
		logger:        logger.With().Str("component", "server").Logger(),
		store:         store,
		nodeService:   nodeService,
		configService: configService,
		taskService:   taskService,
		statusService: statusService,
		userService:   userService,
		listener:      listener,
		mux:           mux,
		grpcServer:    grpcServer,
		httpServer:    router,
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

	// 修改 HTTP 服务器启动方式
	httpServer := &http.Server{
		Handler: s.httpServer,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := httpServer.Serve(httpL); err != nil && err != http.ErrServerClosed {
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
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// if err := s.httpServer.Shutdown(ctx); err != nil {
	// 	s.logger.Error().Err(err).Msg("Error shutting down HTTP server")
	// }

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
