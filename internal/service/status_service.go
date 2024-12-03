package service

import (
	"context"
	"mesh-backend/internal/models"
	"mesh-backend/internal/store/types"
)

type StatusService struct {
	store types.Store
}

func NewStatusService(store types.Store) *StatusService {
	return &StatusService{store: store}
}

func (s *StatusService) GetSystemStatus() (*models.SystemStatus, error) {
	ctx := context.Background()

	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	tasks, err := s.store.ListTasks(ctx)
	if err != nil {
		return nil, err
	}

	onlineCount := 0
	for _, node := range nodes {
		if node.Status == models.NodeStatusOnline {
			onlineCount++
		}
	}

	pendingTasks := 0
	for _, task := range tasks {
		if task.Status == models.TaskStatusPending {
			pendingTasks++
		}
	}

	status := &models.SystemStatus{
		TotalNodes:   len(nodes),
		OnlineNodes:  onlineCount,
		PendingTasks: pendingTasks,
	}

	return status, nil
}
