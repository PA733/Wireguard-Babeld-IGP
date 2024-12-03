package api

import (
	"net/http"
	"strings"

	pb "mesh-backend/api/proto/task"
	"mesh-backend/internal/service"
	"mesh-backend/pkg/logger"

	"google.golang.org/grpc"
)

type MixedHandler struct {
	grpcServer  *grpc.Server
	httpHandler http.Handler
	logger      *logger.Logger
}

func NewMixedHandler(
	grpcServer *grpc.Server,
	httpHandler http.Handler,
	taskService *service.TaskService,
	logger *logger.Logger,
) *MixedHandler {
	// 注册 gRPC 服务
	pb.RegisterTaskServiceServer(grpcServer, taskService)

	return &MixedHandler{
		grpcServer:  grpcServer,
		httpHandler: httpHandler,
		logger:      logger,
	}
}

func (h *MixedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 检查是否是 gRPC 请求
	if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") &&
		strings.HasPrefix(r.URL.Path, "/task") {
		h.grpcServer.ServeHTTP(w, r)
		return
	}

	// 其他请求走 HTTP 处理
	h.httpHandler.ServeHTTP(w, r)
}
