package services

import (
	"context"
	"sync"
	"time"

	pb "mesh-backend/api/proto/status"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/server/middleware"
	"mesh-backend/pkg/store"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StatusService 实现状态管理服务
type StatusService struct {
	pb.UnimplementedStatusServiceServer

	config   *config.ServerConfig
	logger   zerolog.Logger
	store    store.Store
	nodeAuth *middleware.NodeAuthenticator

	// 节点状态管理
	nodeStatuses      map[int32]*pb.NodeStatus
	nodeStatusesMu    sync.RWMutex
	statusSubscribers map[string][]pb.StatusService_SubscribeStatusServer
	subscribersMu     sync.RWMutex
}

// NewStatusService 创建状态服务实例
func NewStatusService(cfg *config.ServerConfig, logger zerolog.Logger, store store.Store, nodeAuth *middleware.NodeAuthenticator) *StatusService {
	return &StatusService{
		config:            cfg,
		logger:            logger.With().Str("service", "status").Logger(),
		store:             store,
		nodeAuth:          nodeAuth,
		nodeStatuses:      make(map[int32]*pb.NodeStatus),
		statusSubscribers: make(map[string][]pb.StatusService_SubscribeStatusServer),
	}
}

// RegisterGRPC 注册gRPC服务
func (s *StatusService) RegisterGRPC(server *grpc.Server) {
	pb.RegisterStatusServiceServer(server, s)
}

// ReportStatus 实现状态上报
func (s *StatusService) ReportStatus(ctx context.Context, req *pb.StatusReport) (*pb.StatusResponse, error) {
	// 验证节点身份
	if !s.nodeAuth.ValidateToken(int(req.NodeId), req.Token) {
		return &pb.StatusResponse{
			Success: false,
			Message: "Invalid credentials",
		}, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	// 更新节点状态
	s.nodeStatusesMu.Lock()
	s.nodeStatuses[req.NodeId] = req.Status
	s.nodeStatusesMu.Unlock()

	// 广播状态更新给订阅者
	s.subscribersMu.RLock()
	for _, subscribers := range s.statusSubscribers {
		for _, subscriber := range subscribers {
			if err := subscriber.Send(req.Status); err != nil {
				s.logger.Error().
					Err(err).
					Int32("node_id", req.NodeId).
					Msg("Failed to send status update to subscriber")
			}
		}
	}
	s.subscribersMu.RUnlock()

	// 保存状态到存储
	if err := s.store.UpdateNodeStatus(int(req.Status.NodeId), &types.NodeStatus{
		NodeID:    int(req.Status.NodeId),
		Hostname:  req.Status.Hostname,
		IPAddress: req.Status.IpAddress,
		Metrics: types.SystemMetrics{
			CPUUsage:    req.Status.Metrics.CpuUsage,
			MemoryUsage: req.Status.Metrics.MemoryUsage,
			DiskUsage:   req.Status.Metrics.DiskUsage,
			Uptime:      req.Status.Metrics.Uptime,
		},
		RunningTasks: req.Status.RunningTasks,
		Status:       req.Status.Status,
		Version:      req.Status.Version,
		Timestamp:    time.Unix(0, req.Status.Timestamp),
	}); err != nil {
		s.logger.Error().
			Err(err).
			Int32("node_id", req.NodeId).
			Msg("Failed to save node status")
	}

	return &pb.StatusResponse{
		Success: true,
		Message: "Status updated successfully",
	}, nil
}

// SubscribeStatus 实现状态订阅
func (s *StatusService) SubscribeStatus(req *pb.StatusSubscribeRequest, stream pb.StatusService_SubscribeStatusServer) error {
	// 验证订阅者身份
	if !s.validateSubscriber(req.Token) {
		return status.Error(codes.Unauthenticated, "invalid subscriber token")
	}

	// 注册订阅者
	s.subscribersMu.Lock()
	s.statusSubscribers[req.Token] = append(s.statusSubscribers[req.Token], stream)
	s.subscribersMu.Unlock()

	// 发送当前所有节点状态
	s.nodeStatusesMu.RLock()
	for _, nodeStatus := range s.nodeStatuses {
		if err := stream.Send(nodeStatus); err != nil {
			s.logger.Error().
				Err(err).
				Msg("Failed to send initial status to subscriber")
		}
	}
	s.nodeStatusesMu.RUnlock()

	// 等待连接断开
	<-stream.Context().Done()

	// 移除订阅者
	s.subscribersMu.Lock()
	subscribers := s.statusSubscribers[req.Token]
	for i, sub := range subscribers {
		if sub == stream {
			s.statusSubscribers[req.Token] = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}
	s.subscribersMu.Unlock()

	return nil
}

// validateSubscriber 验证订阅者身份
func (s *StatusService) validateSubscriber(token string) bool {
	// TODO: 实现订阅者身份验证逻辑
	// 这里可以根据实际需求实现不同的验证方式
	return token != ""
}

// GetNodeStatus 获取指定节点的状态
func (s *StatusService) GetNodeStatus(nodeID int32) (*pb.NodeStatus, bool) {
	s.nodeStatusesMu.RLock()
	defer s.nodeStatusesMu.RUnlock()

	status, exists := s.nodeStatuses[nodeID]
	return status, exists
}

// GetAllNodeStatuses 获取所有节点的状态
func (s *StatusService) GetAllNodeStatuses() map[int32]*pb.NodeStatus {
	s.nodeStatusesMu.RLock()
	defer s.nodeStatusesMu.RUnlock()

	statuses := make(map[int32]*pb.NodeStatus, len(s.nodeStatuses))
	for nodeID, status := range s.nodeStatuses {
		statuses[nodeID] = status
	}
	return statuses
}
