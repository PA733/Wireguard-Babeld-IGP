package store

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"mesh-backend/pkg/types"
)

// MemoryStore 内存存储实现
type MemoryStore struct {
	sync.RWMutex
	nodes       map[int]*types.NodeConfig
	connections map[int]*types.WireguardConnection
	tasks       map[string]*types.Task
	status      map[int]*types.NodeStatus
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:       make(map[int]*types.NodeConfig),
		connections: make(map[int]*types.WireguardConnection),
		tasks:       make(map[string]*types.Task),
		status:      make(map[int]*types.NodeStatus),
	}
}

// CreateNode 创建节点
func (s *MemoryStore) CreateNode(node *types.NodeConfig) error {
	s.Lock()
	defer s.Unlock()

	if _, exists := s.nodes[node.ID]; exists {
		return fmt.Errorf("node %d already exists", node.ID)
	}

	s.nodes[node.ID] = node
	return nil
}

// GetNode 获取节点
func (s *MemoryStore) GetNode(nodeID int) (*types.NodeConfig, error) {
	s.RLock()
	defer s.RUnlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %d not found", nodeID)
	}

	return node, nil
}

// UpdateNode 更新节点
func (s *MemoryStore) UpdateNode(nodeID int, node *types.NodeConfig) error {
	s.Lock()
	defer s.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	s.nodes[nodeID] = node
	return nil
}

// DeleteNode 删除节点
func (s *MemoryStore) DeleteNode(nodeID int) error {
	s.Lock()
	defer s.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	delete(s.nodes, nodeID)
	return nil
}

// ListNodes 列出所有节点
func (s *MemoryStore) ListNodes() ([]*types.NodeConfig, error) {
	s.RLock()
	defer s.RUnlock()

	nodes := make([]*types.NodeConfig, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetOrCreateWireguardConnection 获取或创建Wireguard连接
func (s *MemoryStore) GetOrCreateWireguardConnection(connection *types.WireguardConnection, basePort int) (*types.WireguardConnection, error) {
	if connection == nil {
		return nil, fmt.Errorf("connection cannot be nil")
	}

	var conn types.WireguardConnection

	// 情况1：如果提供了Port，则根据Port查询连接
	if connection.Port != 0 {
		s.RLock()
		for _, c := range s.connections {
			if c.NodeID == connection.NodeID && c.Port == connection.Port {
				conn = *c
				break
			}
		}
		s.RUnlock()
	}

	// 情况2：如果提供了NodeID和PeerID，则根据它们查询连接
	if conn.NodeID == 0 && conn.PeerID == 0 {
		s.RLock()
		nodeID, peerID := connection.NodeID, connection.PeerID
		if nodeID > peerID {
			// ChatGPT 真天才
			nodeID, peerID = peerID, nodeID
		}

		for _, c := range s.connections {
			if c.NodeID == nodeID && c.PeerID == peerID {
				conn = *c
				break
			}
		}
		s.RUnlock()

		if conn.NodeID != 0 && conn.PeerID != 0 {
			return &conn, nil
		}

		// 未找到连接，创建新连接
		// 获取当前最大的端口号
		maxPort := basePort
		s.RLock()
		for _, c := range s.connections {
			if c.Port > maxPort {
				maxPort = c.Port
			}
		}
		s.RUnlock()

		newPort := basePort
		if maxPort > basePort {
			newPort = maxPort + 1
		}

		conn = types.WireguardConnection{
			NodeID: connection.NodeID,
			PeerID: connection.PeerID,
			Port:   newPort,
		}

		s.Lock()
		s.connections[len(s.connections)] = &conn
		s.Unlock()

		return &conn, nil
	}

	// 输入参数无效，返回错误
	return nil, fmt.Errorf("invalid connection parameters; must provide either port, or node_id and peer_id")
}

// UpdateNodeStatus 更新节点状态
func (s *MemoryStore) UpdateNodeStatus(nodeID int, status *types.NodeStatus) error {
	s.Lock()
	defer s.Unlock()

	s.status[nodeID] = status
	return nil
}

// GetNodeStatus 获取节点状态
func (s *MemoryStore) GetNodeStatus(nodeID int) (*types.NodeStatus, error) {
	s.RLock()
	defer s.RUnlock()

	status, exists := s.status[nodeID]
	if !exists {
		return nil, fmt.Errorf("status for node %d not found", nodeID)
	}

	return status, nil
}

// ListNodeStatus 列出所有节点状态
func (s *MemoryStore) ListNodeStatus() ([]*types.NodeStatus, error) {
	s.RLock()
	defer s.RUnlock()

	statuses := make([]*types.NodeStatus, 0, len(s.status))
	for _, status := range s.status {
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// SaveTask 保存任务
func (s *MemoryStore) SaveTask(task *types.Task) error {
	s.Lock()
	defer s.Unlock()

	s.tasks[task.ID] = task
	return nil
}

// GetTask 获取任务
func (s *MemoryStore) GetTask(id string) (*types.Task, error) {
	s.RLock()
	defer s.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

// ListTasks 列出任务
func (s *MemoryStore) ListTasks(filter TaskFilter) ([]*types.Task, error) {
	s.RLock()
	defer s.RUnlock()

	var tasks []*types.Task
	for _, task := range s.tasks {
		if matchesFilter(task, filter) {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

// DeleteTask 删除任务
func (s *MemoryStore) DeleteTask(id string) error {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	delete(s.tasks, id)
	return nil
}

// CleanupTasks 清理过期任务
func (s *MemoryStore) CleanupTasks() error {
	s.Lock()
	defer s.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for id, task := range s.tasks {
		if task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {
			delete(s.tasks, id)
		}
	}
	return nil
}

// Close 关闭存储
func (s *MemoryStore) Close() error {
	return nil
}

// TaskFilter 任务过滤器
type TaskFilter struct {
	NodeID *int
	Status *types.TaskStatus
	Type   *types.TaskType
}

// matchesFilter 检查任务是否匹配过滤条件
func matchesFilter(task *types.Task, filter TaskFilter) bool {
	if filter.NodeID != nil {
		nodeIDStr, ok := task.Params["node_id"]
		if !ok {
			return false
		}
		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil || nodeID != *filter.NodeID {
			return false
		}
	}

	if filter.Status != nil && task.Status != *filter.Status {
		return false
	}

	if filter.Type != nil && task.Type != *filter.Type {
		return false
	}

	return true
}
