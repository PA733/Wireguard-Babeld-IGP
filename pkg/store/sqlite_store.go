package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mesh-backend/pkg/types"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore 实现了基于 SQLite 的存储
type SQLiteStore struct {
	db *sql.DB
}

// DefaultSQLiteConfig 返回默认的 SQLite 配置
func DefaultSQLiteConfig(path string) *SQLiteConfig {
	return &SQLiteConfig{
		Path:            path,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute * 30,
	}
}

// NewSQLiteStore 创建一个新的 SQLite 存储实例
func NewSQLiteStore(config *SQLiteConfig) (*SQLiteStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite3", config.Path+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	store := &SQLiteStore{db: db}

	// 初始化表结构
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init tables: %w", err)
	}

	return store, nil
}

// Close 关闭存储
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// initTables 初始化数据库表
func (s *SQLiteStore) initTables() error {
	// 开启事务
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 创建节点表
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id INTEGER PRIMARY KEY,
			wireguard TEXT NOT NULL,
			babel TEXT NOT NULL,
			node_info TEXT NOT NULL,
			network TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create nodes table: %w", err)
	}

	// 创建节点状态表
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS node_status (
			node_id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			start_time TEXT NOT NULL,
			system TEXT NOT NULL,
			network TEXT NOT NULL,
			last_error TEXT,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("create node_status table: %w", err)
	}

	// 创建任务表
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY,
			node_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			data TEXT NOT NULL,
			error TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("create tasks table: %w", err)
	}

	// 创建索引
	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS idx_nodes_updated_at ON nodes(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_node_status_updated_at ON node_status(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_node_id ON tasks(node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_updated_at ON tasks(updated_at)`,
	} {
		if _, err := tx.Exec(idx); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// CreateNode 创建节点
func (s *SQLiteStore) CreateNode(ctx context.Context, node *types.NodeConfig) error {
	if node == nil {
		return fmt.Errorf("node is required")
	}

	// 序列化配置
	wireguardJSON, err := json.Marshal(node.WireGuard)
	if err != nil {
		return fmt.Errorf("marshal wireguard config: %w", err)
	}

	nodeInfoJSON, err := json.Marshal(node.NodeInfo)
	if err != nil {
		return fmt.Errorf("marshal node info: %w", err)
	}

	networkJSON, err := json.Marshal(node.Network)
	if err != nil {
		return fmt.Errorf("marshal network config: %w", err)
	}

	now := time.Now().Unix()

	// 插入记录
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO nodes (
			id, wireguard, babel, node_info, network, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		node.NodeInfo.ID,
		string(wireguardJSON),
		node.Babel,
		string(nodeInfoJSON),
		string(networkJSON),
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}

	return nil
}

// GetNode 获取节点
func (s *SQLiteStore) GetNode(ctx context.Context, nodeID int) (*types.NodeConfig, error) {
	var (
		wireguardJSON string
		babel         string
		nodeInfoJSON  string
		networkJSON   string
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT wireguard, babel, node_info, network
		FROM nodes WHERE id = ?
	`, nodeID).Scan(&wireguardJSON, &babel, &nodeInfoJSON, &networkJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node %d not found", nodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("query node: %w", err)
	}

	node := &types.NodeConfig{
		Babel: babel,
	}

	// 反序列化配置
	if err := json.Unmarshal([]byte(wireguardJSON), &node.WireGuard); err != nil {
		return nil, fmt.Errorf("unmarshal wireguard config: %w", err)
	}

	if err := json.Unmarshal([]byte(nodeInfoJSON), &node.NodeInfo); err != nil {
		return nil, fmt.Errorf("unmarshal node info: %w", err)
	}

	if err := json.Unmarshal([]byte(networkJSON), &node.Network); err != nil {
		return nil, fmt.Errorf("unmarshal network config: %w", err)
	}

	return node, nil
}

// UpdateNodeStatus 更新节点状态
func (s *SQLiteStore) UpdateNodeStatus(ctx context.Context, nodeID int, status *types.NodeStatus) error {
	if status == nil {
		return fmt.Errorf("status is required")
	}

	// 序列化状态
	systemJSON, err := json.Marshal(status.System)
	if err != nil {
		return fmt.Errorf("marshal system status: %w", err)
	}

	networkJSON, err := json.Marshal(status.Network)
	if err != nil {
		return fmt.Errorf("marshal network status: %w", err)
	}

	now := time.Now().Unix()

	// 更新或插入状态
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO node_status (
			node_id, name, version, start_time, system, network,
			last_error, last_error_time, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			name = excluded.name,
			version = excluded.version,
			start_time = excluded.start_time,
			system = excluded.system,
			network = excluded.network,
			last_error = excluded.last_error,
			last_error_time = excluded.last_error_time,
			updated_at = excluded.updated_at
	`,
		nodeID,
		status.Name,
		status.Version,
		status.StartTime,
		string(systemJSON),
		string(networkJSON),
		status.LastError,
		status.LastErrorTime,
		now,
	)

	if err != nil {
		return fmt.Errorf("upsert node status: %w", err)
	}

	return nil
}

// GetNodeStatus 获取节点状态
func (s *SQLiteStore) GetNodeStatus(ctx context.Context, nodeID int) (*types.NodeStatus, error) {
	var (
		status      types.NodeStatus
		systemJSON  string
		networkJSON string
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT name, version, start_time, system, network,
			last_error, last_error_time
		FROM node_status WHERE node_id = ?
	`, nodeID).Scan(
		&status.Name,
		&status.Version,
		&status.StartTime,
		&systemJSON,
		&networkJSON,
		&status.LastError,
		&status.LastErrorTime,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("status for node %d not found", nodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("query node status: %w", err)
	}

	status.ID = nodeID

	// 反序列化状态
	if err := json.Unmarshal([]byte(systemJSON), &status.System); err != nil {
		return nil, fmt.Errorf("unmarshal system status: %w", err)
	}

	if err := json.Unmarshal([]byte(networkJSON), &status.Network); err != nil {
		return nil, fmt.Errorf("unmarshal network status: %w", err)
	}

	return &status, nil
}

// ListNodes 列出所有节点
func (s *SQLiteStore) ListNodes(ctx context.Context) ([]*types.NodeConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, wireguard, babel, node_info, network
		FROM nodes ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*types.NodeConfig
	for rows.Next() {
		var (
			nodeID        int
			wireguardJSON string
			babel         string
			nodeInfoJSON  string
			networkJSON   string
		)

		if err := rows.Scan(&nodeID, &wireguardJSON, &babel, &nodeInfoJSON, &networkJSON); err != nil {
			return nil, fmt.Errorf("scan node row: %w", err)
		}

		node := &types.NodeConfig{
			Babel: babel,
		}

		// 反序列化配置
		if err := json.Unmarshal([]byte(wireguardJSON), &node.WireGuard); err != nil {
			return nil, fmt.Errorf("unmarshal wireguard config: %w", err)
		}

		if err := json.Unmarshal([]byte(nodeInfoJSON), &node.NodeInfo); err != nil {
			return nil, fmt.Errorf("unmarshal node info: %w", err)
		}

		if err := json.Unmarshal([]byte(networkJSON), &node.Network); err != nil {
			return nil, fmt.Errorf("unmarshal network config: %w", err)
		}

		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node rows: %w", err)
	}

	return nodes, nil
}

// ListNodeStatus 列出所有节点状态
func (s *SQLiteStore) ListNodeStatus(ctx context.Context) ([]*types.NodeStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT node_id, name, version, start_time, system, network,
			last_error, last_error_time
		FROM node_status ORDER BY node_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query node status: %w", err)
	}
	defer rows.Close()

	var statuses []*types.NodeStatus
	for rows.Next() {
		var (
			status      types.NodeStatus
			systemJSON  string
			networkJSON string
		)

		if err := rows.Scan(
			&status.ID,
			&status.Name,
			&status.Version,
			&status.StartTime,
			&systemJSON,
			&networkJSON,
			&status.LastError,
			&status.LastErrorTime,
		); err != nil {
			return nil, fmt.Errorf("scan status row: %w", err)
		}

		// 反序列化状态
		if err := json.Unmarshal([]byte(systemJSON), &status.System); err != nil {
			return nil, fmt.Errorf("unmarshal system status: %w", err)
		}

		if err := json.Unmarshal([]byte(networkJSON), &status.Network); err != nil {
			return nil, fmt.Errorf("unmarshal network status: %w", err)
		}

		statuses = append(statuses, &status)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status rows: %w", err)
	}

	return statuses, nil
}

// Cleanup 清理过期数据
func (s *SQLiteStore) Cleanup(ctx context.Context) error {
	// 清理7天前的过期节点状态
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM node_status
		WHERE updated_at < ?
	`, time.Now().AddDate(0, 0, -7).Unix())

	if err != nil {
		return fmt.Errorf("cleanup node status: %w", err)
	}
	return nil
}

// CreateTask 创建任务
func (s *SQLiteStore) CreateTask(ctx context.Context, task *types.Task) error {
	return fmt.Errorf("task operations not implemented")
}

// GetTask 获取任务
func (s *SQLiteStore) GetTask(ctx context.Context, taskID string) (*types.Task, error) {
	return nil, fmt.Errorf("task operations not implemented")
}

// UpdateTask 更新任务
func (s *SQLiteStore) UpdateTask(ctx context.Context, taskID string, task *types.Task) error {
	return fmt.Errorf("task operations not implemented")
}

// DeleteTask 删除任务
func (s *SQLiteStore) DeleteTask(ctx context.Context, taskID string) error {
	return fmt.Errorf("task operations not implemented")
}

// ListTasks 列出任务
func (s *SQLiteStore) ListTasks(ctx context.Context, filter TaskFilter) ([]*types.Task, error) {
	return nil, fmt.Errorf("task operations not implemented")
}

// DeleteNode 删除节点
func (s *SQLiteStore) DeleteNode(ctx context.Context, nodeID int) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM nodes WHERE id = ?", nodeID)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("node not found: %d", nodeID)
	}
	return nil
}

// UpdateNode 更新节点
func (s *SQLiteStore) UpdateNode(ctx context.Context, nodeID int, node *types.NodeConfig) error {
	if node == nil {
		return fmt.Errorf("node is required")
	}

	wireguardJSON, err := json.Marshal(node.WireGuard)
	if err != nil {
		return fmt.Errorf("marshal wireguard config: %w", err)
	}

	nodeInfoJSON, err := json.Marshal(node.NodeInfo)
	if err != nil {
		return fmt.Errorf("marshal node info: %w", err)
	}

	networkJSON, err := json.Marshal(node.Network)
	if err != nil {
		return fmt.Errorf("marshal network config: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE nodes SET wireguard = ?, babel = ?, node_info = ?, network = ?, updated_at = ?
		WHERE id = ?
	`, string(wireguardJSON), node.Babel, string(nodeInfoJSON), string(networkJSON), time.Now().Unix(), nodeID)

	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("node not found: %d", nodeID)
	}
	return nil
}
