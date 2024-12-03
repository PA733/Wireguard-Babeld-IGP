package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mesh-backend/pkg/types"
)

// MemoryStore 内存存储实现
type MemoryStore struct {
	nodes  map[int]*types.NodeConfig
	tasks  map[string]*types.Task
	status map[int]*types.NodeStatus
	mu     sync.RWMutex
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore() (Store, error) {
	return &MemoryStore{
		nodes:  make(map[int]*types.NodeConfig),
		tasks:  make(map[string]*types.Task),
		status: make(map[int]*types.NodeStatus),
	}, nil
}

// Node operations

func (s *MemoryStore) CreateNode(ctx context.Context, node *types.NodeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[node.NodeInfo.ID]; exists {
		return fmt.Errorf("node %d already exists", node.NodeInfo.ID)
	}

	s.nodes[node.NodeInfo.ID] = node
	return nil
}

func (s *MemoryStore) GetNode(ctx context.Context, nodeID int) (*types.NodeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %d not found", nodeID)
	}

	return node, nil
}

func (s *MemoryStore) UpdateNode(ctx context.Context, nodeID int, node *types.NodeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	s.nodes[nodeID] = node
	return nil
}

func (s *MemoryStore) DeleteNode(ctx context.Context, nodeID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	delete(s.nodes, nodeID)
	return nil
}

func (s *MemoryStore) ListNodes(ctx context.Context) ([]*types.NodeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]*types.NodeConfig, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// Task operations

func (s *MemoryStore) CreateTask(ctx context.Context, task *types.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}

	s.tasks[task.ID] = task
	return nil
}

func (s *MemoryStore) GetTask(ctx context.Context, taskID string) (*types.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

func (s *MemoryStore) UpdateTask(ctx context.Context, taskID string, task *types.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	s.tasks[taskID] = task
	return nil
}

func (s *MemoryStore) DeleteTask(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	delete(s.tasks, taskID)
	return nil
}

func (s *MemoryStore) ListTasks(ctx context.Context, filter TaskFilter) ([]*types.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*types.Task
	for _, task := range s.tasks {
		if matchesFilter(task, filter) {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// Status operations

func (s *MemoryStore) UpdateNodeStatus(ctx context.Context, nodeID int, status *types.NodeStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status[nodeID] = status
	return nil
}

func (s *MemoryStore) GetNodeStatus(ctx context.Context, nodeID int) (*types.NodeStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status, exists := s.status[nodeID]
	if !exists {
		return nil, fmt.Errorf("status for node %d not found", nodeID)
	}

	return status, nil
}

func (s *MemoryStore) ListNodeStatus(ctx context.Context) ([]*types.NodeStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]*types.NodeStatus, 0, len(s.status))
	for _, status := range s.status {
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// Maintenance

func (s *MemoryStore) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清理过期的任务（24小时前完成的任务）
	cutoff := time.Now().Add(-24 * time.Hour)
	for id, task := range s.tasks {
		if task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {
			delete(s.tasks, id)
		}
	}

	// 清理过期的状态（1小时未更新的状态）
	statusCutoff := time.Now().Add(-1 * time.Hour)
	for id, status := range s.status {
		lastUpdate, err := time.Parse(time.RFC3339, status.LastErrorTime)
		if err == nil && lastUpdate.Before(statusCutoff) {
			delete(s.status, id)
		}
	}

	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}

// Helper functions

func matchesFilter(task *types.Task, filter TaskFilter) bool {
	if filter.NodeID != nil {
		nodeID, ok := task.Params["node_id"].(int)
		if !ok || nodeID != *filter.NodeID {
			return false
		}
	}

	if filter.Status != nil && task.Status != *filter.Status {
		return false
	}

	if filter.Type != nil && task.Type != *filter.Type {
		return false
	}

	if filter.StartTime != nil && task.CreatedAt.Unix() < *filter.StartTime {
		return false
	}

	if filter.EndTime != nil && task.CreatedAt.Unix() > *filter.EndTime {
		return false
	}

	return true
}
