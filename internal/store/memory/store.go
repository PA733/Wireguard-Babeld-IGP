package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"mesh-backend/internal/models"
)

var (
	ErrNodeNotFound = errors.New("node not found")
)

type Store struct {
	nodes  map[int]*models.Node
	tasks  map[string]*models.Task
	lastID int32 // 用于生成自增ID
	mu     sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		nodes:  make(map[int]*models.Node),
		tasks:  make(map[string]*models.Task),
		lastID: 0,
	}
}

// 生成新的节点ID
func (s *Store) nextNodeID() int {
	return int(atomic.AddInt32(&s.lastID, 1))
}

// Node 操作
func (s *Store) CreateNode(ctx context.Context, node *models.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node.ID == 0 {
		node.ID = s.nextNodeID()
	}

	s.nodes[node.ID] = node
	return nil
}

func (s *Store) GetNode(ctx context.Context, id int) (*models.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if node, exists := s.nodes[id]; exists {
		return node, nil
	}
	return nil, ErrNodeNotFound
}

func (s *Store) ListNodes(ctx context.Context) ([]*models.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes := make([]*models.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (s *Store) DeleteNode(ctx context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, id)
	return nil
}

// Task 操作
func (s *Store) CreateTask(ctx context.Context, task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	return nil
}

func (s *Store) GetTask(ctx context.Context, id string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if task, exists := s.tasks[id]; exists {
		return task, nil
	}
	return nil, fmt.Errorf("task not found: %s", id)
}

func (s *Store) UpdateTask(ctx context.Context, task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tasks[task.ID]; !exists {
		return fmt.Errorf("task not found: %s", task.ID)
	}
	s.tasks[task.ID] = task
	return nil
}

func (s *Store) ListTasks(ctx context.Context) ([]*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]*models.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (s *Store) DeleteTask(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
	return nil
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) UpdateNode(ctx context.Context, node *models.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[node.ID]; !exists {
		return fmt.Errorf("node not found: %d", node.ID)
	}

	s.nodes[node.ID] = node
	return nil
}
