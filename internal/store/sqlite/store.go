package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"mesh-backend/internal/models"
	"mesh-backend/internal/store/types"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func NewStore(cfg types.SQLiteConfig) (*Store, error) {
	// 确保目录存在
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", cfg.Path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// 创建表
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) CreateNode(ctx context.Context, node *models.Node) error {
	query := `
		INSERT INTO nodes (
			name, ipv4_prefix, ipv6_prefix, link_local_addr, endpoint,
			public_key, private_key, status, token, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err := s.db.QueryRowContext(ctx, query,
		node.Name, node.IPv4Prefix, node.IPv6Prefix, node.LinkLocalAddr,
		node.Endpoint, node.PublicKey, node.PrivateKey, node.Status,
		node.Token, node.CreatedAt, node.UpdatedAt,
	).Scan(&node.ID)

	if err != nil {
		return fmt.Errorf("inserting node: %w", err)
	}
	return nil
}

func (s *Store) GetNode(ctx context.Context, id int) (*models.Node, error) {
	node := &models.Node{}
	query := `SELECT * FROM nodes WHERE id = ?`

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&node.ID, &node.Name, &node.IPv4Prefix, &node.IPv6Prefix,
		&node.LinkLocalAddr, &node.Endpoint, &node.PublicKey, &node.PrivateKey,
		&node.Status, &node.Token, &node.CreatedAt, &node.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying node: %w", err)
	}
	return node, nil
}

func (s *Store) ListNodes(ctx context.Context) ([]*models.Node, error) {
	query := `SELECT * FROM nodes`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		node := &models.Node{}
		err := rows.Scan(
			&node.ID, &node.Name, &node.IPv4Prefix, &node.IPv6Prefix,
			&node.LinkLocalAddr, &node.Endpoint, &node.PublicKey, &node.PrivateKey,
			&node.Status, &node.Token, &node.CreatedAt, &node.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning node: %w", err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (s *Store) UpdateNode(ctx context.Context, node *models.Node) error {
	query := `
		UPDATE nodes SET 
			name = ?, ipv4_prefix = ?, ipv6_prefix = ?, link_local_addr = ?,
			endpoint = ?, public_key = ?, private_key = ?, status = ?,
			token = ?, updated_at = ?
		WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query,
		node.Name, node.IPv4Prefix, node.IPv6Prefix, node.LinkLocalAddr,
		node.Endpoint, node.PublicKey, node.PrivateKey, node.Status,
		node.Token, node.UpdatedAt, node.ID,
	)
	if err != nil {
		return fmt.Errorf("updating node: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("node not found: %d", node.ID)
	}
	return nil
}

func (s *Store) DeleteNode(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM nodes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting node: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("node not found: %d", id)
	}
	return nil
}

// Task operations
func (s *Store) CreateTask(ctx context.Context, task *models.Task) error {
	query := `
		INSERT INTO tasks (
			id, node_id, type, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		task.ID, task.NodeID, task.Type, task.Status,
		task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting task: %w", err)
	}
	return nil
}

func (s *Store) GetTask(ctx context.Context, id string) (*models.Task, error) {
	task := &models.Task{}
	query := `SELECT * FROM tasks WHERE id = ?`

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID, &task.NodeID, &task.Type, &task.Status,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying task: %w", err)
	}
	return task, nil
}

func (s *Store) ListTasks(ctx context.Context) ([]*models.Task, error) {
	query := `SELECT * FROM tasks`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		task := &models.Task{}
		err := rows.Scan(
			&task.ID, &task.NodeID, &task.Type, &task.Status,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning task: %w", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (s *Store) UpdateTask(ctx context.Context, task *models.Task) error {
	query := `
		UPDATE tasks SET 
			node_id = ?, type = ?, status = ?, updated_at = ?
		WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query,
		task.NodeID, task.Type, task.Status,
		task.UpdatedAt, task.ID,
	)
	if err != nil {
		return fmt.Errorf("updating task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("task not found: %s", task.ID)
	}
	return nil
}

func (s *Store) DeleteTask(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
