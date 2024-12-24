package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"mesh-backend/pkg/types"

	_ "modernc.org/sqlite"
)

// SQLiteStore SQLite存储实现
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 创建SQLite存储实例
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	// 连接数据库
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	// 创建存储实例
	store := &SQLiteStore{db: db}

	// 初始化数据库
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing database: %w", err)
	}

	return store, nil
}

// initialize 初始化数据库
func (s *SQLiteStore) initialize() error {
	// 创建节点表
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			token TEXT NOT NULL,
			ipv4 TEXT NOT NULL,
			ipv6 TEXT NOT NULL,
			peers TEXT NOT NULL,
			endpoints TEXT NOT NULL,
			public_key TEXT NOT NULL,
			private_key TEXT NOT NULL,
			wireguard TEXT NOT NULL,
			babel TEXT NOT NULL,
			network TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("creating nodes table: %w", err)
	}

	// 创建节点状态表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS node_status (
			node_id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			start_time DATETIME NOT NULL,
			last_seen DATETIME NOT NULL,
			last_error TEXT,
			last_error_time TEXT,
			status TEXT NOT NULL,
			system TEXT NOT NULL,
			network TEXT NOT NULL,
			FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating node_status table: %w", err)
	}

	// 创建任务表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			params TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME
		)
	`)
	if err != nil {
		return fmt.Errorf("creating tasks table: %w", err)
	}

	// 创建Wireguard连接表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS wireguard_connections (
			node_id INTEGER NOT NULL,
			peer_id INTEGER NOT NULL,
			port INTEGER NOT NULL,
			UNIQUE (node_id, peer_id),
			UNIQUE (port),
			FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
			FOREIGN KEY (peer_id) REFERENCES nodes(id) ON DELETE CASCADE,
			CHECK (node_id != peer_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating tasks table: %w", err)
	}

	// 创建索引
	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS idx_nodes_created_at ON nodes(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_node_status_last_seen ON node_status(last_seen)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_wireguard_connections_node_peer ON wireguard_connections(node_id, peer_id)`,
	} {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
	}

	return nil
}

// SaveTask 保存任务
func (s *SQLiteStore) SaveTask(task *types.Task) error {
	// 序列化参数
	params, err := json.Marshal(task.Params)
	if err != nil {
		return fmt.Errorf("marshaling params: %w", err)
	}

	// 保存任务
	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO tasks (
			id, type, status, params, created_at, updated_at, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		task.ID,
		task.Type,
		task.Status,
		string(params),
		task.CreatedAt,
		task.UpdatedAt,
		task.StartedAt,
		task.CompletedAt,
	)
	return err
}

// GetTask 获取任务
func (s *SQLiteStore) GetTask(id string) (*types.Task, error) {
	var task types.Task
	var paramsStr string
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, type, status, params, created_at, updated_at, started_at, completed_at
		FROM tasks WHERE id = ?
	`, id).Scan(
		&task.ID,
		&task.Type,
		&task.Status,
		&paramsStr,
		&task.CreatedAt,
		&task.UpdatedAt,
		&startedAt,
		&completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	// 解析参数
	if err := json.Unmarshal([]byte(paramsStr), &task.Params); err != nil {
		return nil, fmt.Errorf("unmarshaling params: %w", err)
	}

	// 处理可选时间字段
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	return &task, nil
}

// ListTasks 列出任务
func (s *SQLiteStore) ListTasks(filter TaskFilter) ([]*types.Task, error) {
	// 构建查询
	query := `
		SELECT id, type, status, params, created_at, updated_at, started_at, completed_at
		FROM tasks WHERE 1=1
	`
	var args []interface{}

	if filter.NodeID != nil {
		query += " AND json_extract(params, '$.node_id') = ?"
		args = append(args, *filter.NodeID)
	}
	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, *filter.Status)
	}
	if filter.Type != nil {
		query += " AND type = ?"
		args = append(args, *filter.Type)
	}

	// 执行查询
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 处理结果
	var tasks []*types.Task
	for rows.Next() {
		var task types.Task
		var paramsStr string
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.Type,
			&task.Status,
			&paramsStr,
			&task.CreatedAt,
			&task.UpdatedAt,
			&startedAt,
			&completedAt,
		)
		if err != nil {
			return nil, err
		}

		// 解析参数
		if err := json.Unmarshal([]byte(paramsStr), &task.Params); err != nil {
			return nil, fmt.Errorf("unmarshaling params: %w", err)
		}

		// 处理可选时间字段
		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}

// DeleteTask 删除任务
func (s *SQLiteStore) DeleteTask(id string) error {
	result, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	return nil
}

// CleanupTasks 清理过期任务
func (s *SQLiteStore) CleanupTasks() error {
	cutoff := time.Now().Add(-24 * time.Hour)
	_, err := s.db.Exec("DELETE FROM tasks WHERE completed_at < ?", cutoff)
	return err
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// CreateNode 创建节点
func (s *SQLiteStore) CreateNode(node *types.NodeConfig) error {
	// 序列化配置
	wireguardJSON, err := json.Marshal(node.WireGuard)
	if err != nil {
		return fmt.Errorf("marshaling wireguard config: %w", err)
	}

	peersJSON, err := json.Marshal(node.Peers)
	if err != nil {
		return fmt.Errorf("marshaling peers: %w", err)
	}

	endpointsJSON, err := json.Marshal(node.Endpoints)
	if err != nil {
		return fmt.Errorf("marshaling endpoints: %w", err)
	}

	networkJSON, err := json.Marshal(node.Network)
	if err != nil {
		return fmt.Errorf("marshaling network config: %w", err)
	}

	// 插入记录
	_, err = s.db.Exec(`
		INSERT INTO nodes (
			id, name, token, ipv4, ipv6, peers, endpoints, public_key, private_key,
			wireguard, babel, network, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		node.ID,
		node.Name,
		node.Token,
		node.IPv4,
		node.IPv6,
		string(peersJSON),
		string(endpointsJSON),
		node.PublicKey,
		node.PrivateKey,
		string(wireguardJSON),
		node.Babel,
		string(networkJSON),
		node.CreatedAt,
		node.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("inserting node: %w", err)
	}

	return nil
}

// GetNode 获取节点
func (s *SQLiteStore) GetNode(nodeID int) (*types.NodeConfig, error) {
	var node types.NodeConfig
	var peersJSON, endpointsJSON, wireguardJSON, networkJSON string

	err := s.db.QueryRow(`
		SELECT id, name, token, ipv4, ipv6, peers, endpoints, public_key, private_key,
			wireguard, babel, network, created_at, updated_at
		FROM nodes WHERE id = ?
	`, nodeID).Scan(
		&node.ID,
		&node.Name,
		&node.Token,
		&node.IPv4,
		&node.IPv6,
		&peersJSON,
		&endpointsJSON,
		&node.PublicKey,
		&node.PrivateKey,
		&wireguardJSON,
		&node.Babel,
		&networkJSON,
		&node.CreatedAt,
		&node.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node %d not found", nodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("querying node: %w", err)
	}

	// 反序列化配置
	if err := json.Unmarshal([]byte(peersJSON), &node.Peers); err != nil {
		return nil, fmt.Errorf("unmarshaling peers: %w", err)
	}

	if err := json.Unmarshal([]byte(endpointsJSON), &node.Endpoints); err != nil {
		return nil, fmt.Errorf("unmarshaling endpoints: %w", err)
	}

	if err := json.Unmarshal([]byte(wireguardJSON), &node.WireGuard); err != nil {
		return nil, fmt.Errorf("unmarshaling wireguard config: %w", err)
	}

	if err := json.Unmarshal([]byte(networkJSON), &node.Network); err != nil {
		return nil, fmt.Errorf("unmarshaling network config: %w", err)
	}

	return &node, nil
}

// UpdateNode 更新节点
func (s *SQLiteStore) UpdateNode(nodeID int, node *types.NodeConfig) error {
	// 序列化配置
	wireguardJSON, err := json.Marshal(node.WireGuard)
	if err != nil {
		return fmt.Errorf("marshaling wireguard config: %w", err)
	}

	peersJSON, err := json.Marshal(node.Peers)
	if err != nil {
		return fmt.Errorf("marshaling peers: %w", err)
	}

	endpointsJSON, err := json.Marshal(node.Endpoints)
	if err != nil {
		return fmt.Errorf("marshaling endpoints: %w", err)
	}

	networkJSON, err := json.Marshal(node.Network)
	if err != nil {
		return fmt.Errorf("marshaling network config: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE nodes SET
			name = ?,
			token = ?,
			ipv4 = ?,
			ipv6 = ?,
			peers = ?,
			endpoints = ?,
			public_key = ?,
			private_key = ?,
			wireguard = ?,
			babel = ?,
			network = ?,
			updated_at = ?
		WHERE id = ?
	`,
		node.Name,
		node.Token,
		node.IPv4,
		node.IPv6,
		string(peersJSON),
		string(endpointsJSON),
		node.PublicKey,
		node.PrivateKey,
		string(wireguardJSON),
		node.Babel,
		string(networkJSON),
		node.UpdatedAt,
		nodeID,
	)

	if err != nil {
		return fmt.Errorf("updating node: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("node %d not found", nodeID)
	}

	return nil
}

// DeleteNode 删除节点
func (s *SQLiteStore) DeleteNode(nodeID int) error {
	result, err := s.db.Exec("DELETE FROM nodes WHERE id = ?", nodeID)
	if err != nil {
		return fmt.Errorf("deleting node: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("node %d not found", nodeID)
	}

	return nil
}

// ListNodes 列出所有节点
func (s *SQLiteStore) ListNodes() ([]*types.NodeConfig, error) {
	rows, err := s.db.Query(`
		SELECT id, name, token, ipv4, ipv6, peers, endpoints, public_key, private_key,
			wireguard, babel, network, created_at, updated_at
		FROM nodes ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("querying nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*types.NodeConfig
	for rows.Next() {
		var node types.NodeConfig
		var peersJSON, endpointsJSON, wireguardJSON, networkJSON string

		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.Token,
			&node.IPv4,
			&node.IPv6,
			&peersJSON,
			&endpointsJSON,
			&node.PublicKey,
			&node.PrivateKey,
			&wireguardJSON,
			&node.Babel,
			&networkJSON,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning node row: %w", err)
		}

		// 反序列化配置
		if err := json.Unmarshal([]byte(peersJSON), &node.Peers); err != nil {
			return nil, fmt.Errorf("unmarshaling peers: %w", err)
		}

		if err := json.Unmarshal([]byte(endpointsJSON), &node.Endpoints); err != nil {
			return nil, fmt.Errorf("unmarshaling endpoints: %w", err)
		}

		if err := json.Unmarshal([]byte(wireguardJSON), &node.WireGuard); err != nil {
			return nil, fmt.Errorf("unmarshaling wireguard config: %w", err)
		}

		if err := json.Unmarshal([]byte(networkJSON), &node.Network); err != nil {
			return nil, fmt.Errorf("unmarshaling network config: %w", err)
		}

		nodes = append(nodes, &node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating node rows: %w", err)
	}

	return nodes, nil
}

// UpdateNodeStatus 更新节点状态
func (s *SQLiteStore) UpdateNodeStatus(nodeID int, status *types.NodeStatus) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO node_status (
			node_id, name, version, start_time, last_seen,
			last_error, last_error_time, status, system, network
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		nodeID,
		status.Name,
		status.Version,
		status.StartTime,
		status.LastSeen,
		status.LastError,
		status.LastErrorTime,
		status.Status,
		status.System,
		status.Network,
	)

	if err != nil {
		return fmt.Errorf("upserting node status: %w", err)
	}

	return nil
}

// GetNodeStatus 获取节点状态
func (s *SQLiteStore) GetNodeStatus(nodeID int) (*types.NodeStatus, error) {
	var status types.NodeStatus

	err := s.db.QueryRow(`
		SELECT node_id, name, version, start_time, last_seen,
			last_error, last_error_time, status, system, network
		FROM node_status WHERE node_id = ?
	`, nodeID).Scan(
		&status.ID,
		&status.Name,
		&status.Version,
		&status.StartTime,
		&status.LastSeen,
		&status.LastError,
		&status.LastErrorTime,
		&status.Status,
		&status.System,
		&status.Network,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("status for node %d not found", nodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("querying node status: %w", err)
	}

	return &status, nil
}

// ListNodeStatus 列出所有节点状态
func (s *SQLiteStore) ListNodeStatus() ([]*types.NodeStatus, error) {
	rows, err := s.db.Query(`
		SELECT node_id, name, version, start_time, last_seen,
			last_error, last_error_time, status, system, network
		FROM node_status ORDER BY node_id
	`)
	if err != nil {
		return nil, fmt.Errorf("querying node status: %w", err)
	}
	defer rows.Close()

	var statuses []*types.NodeStatus
	for rows.Next() {
		var status types.NodeStatus

		err := rows.Scan(
			&status.ID,
			&status.Name,
			&status.Version,
			&status.StartTime,
			&status.LastSeen,
			&status.LastError,
			&status.LastErrorTime,
			&status.Status,
			&status.System,
			&status.Network,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning status row: %w", err)
		}

		statuses = append(statuses, &status)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating status rows: %w", err)
	}

	return statuses, nil
}

// GetOrCreateWireguardConnection 获取或创建Wireguard连接
func (s *SQLiteStore) GetOrCreateWireguardConnection(connection *types.WireguardConnection, basePort int) (*types.WireguardConnection, error) {
	if connection == nil {
		return nil, fmt.Errorf("connection cannot be nil")
	}

	var conn types.WireguardConnection

	// 情况1：如果提供了Port，则根据Port查询连接
	if connection.Port != 0 {
		err := s.db.QueryRow(`
			SELECT node_id, peer_id, port FROM wireguard_connections WHERE port = ?
		`, connection.Port).Scan(&conn.NodeID, &conn.PeerID, &conn.Port)
		if err == nil {
			// 找到了现有的连接
			return &conn, nil
		} else if err != sql.ErrNoRows {
			// 查询出错
			return nil, fmt.Errorf("querying wireguard connection by port: %w", err)
		} else {
			// 未找到连接，返回错误
			return nil, fmt.Errorf("wireguard connection not found with port %d", connection.Port)
		}
	}

	// 情况2：如果提供了NodeID和PeerID，则根据它们查询连接
	if connection.NodeID != 0 && connection.PeerID != 0 {
		err := s.db.QueryRow(`
			SELECT node_id, peer_id, port FROM wireguard_connections 
			WHERE (node_id = ? AND peer_id = ?) 
			OR (node_id = ? AND peer_id = ?)
		`,
			connection.NodeID, connection.PeerID,
			connection.PeerID, connection.NodeID,
		).Scan(&conn.NodeID, &conn.PeerID, &conn.Port)
		if err == nil {
			// 找到了现有的连接
			return &conn, nil
		} else if err != sql.ErrNoRows {
			// 查询出错
			return nil, fmt.Errorf("querying wireguard connection by node_id and peer_id: %w", err)
		}

		// 未找到连接，需要创建新的连接
		// 获取当前数据库中最大的端口号
		var maxPort int
		err = s.db.QueryRow(`SELECT COALESCE(MAX(port), 0) FROM wireguard_connections`).Scan(&maxPort)
		if err != nil {
			return nil, fmt.Errorf("getting max port from wireguard_connections: %w", err)
		}

		// 新的端口号为 max(basePort, maxPortInDB) + 1
		newPort := basePort
		if maxPort >= basePort {
			newPort = maxPort + 1
		}

		// 创建新的连接记录
		_, err = s.db.Exec(`
			INSERT INTO wireguard_connections (node_id, peer_id, port) VALUES (?, ?, ?)
		`, connection.NodeID, connection.PeerID, newPort)
		if err != nil {
			return nil, fmt.Errorf("inserting new wireguard connection: %w", err)
		}

		// 返回新的连接
		conn.NodeID = connection.NodeID
		conn.PeerID = connection.PeerID
		conn.Port = newPort
		return &conn, nil
	}

	// 输入参数无效，返回错误
	return nil, fmt.Errorf("invalid connection parameters; must provide either port, or node_id and peer_id")
}
