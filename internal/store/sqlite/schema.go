package sqlite

const schema = `
-- 节点表
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY,
    wireguard TEXT NOT NULL,
    babel TEXT NOT NULL,
    node_info TEXT NOT NULL,
    network TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 任务表
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    node_id INTEGER NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (node_id) REFERENCES nodes(id)
);
`
