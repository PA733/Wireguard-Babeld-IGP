package types

import "time"

// NodeConfig 节点配置
type NodeConfig struct {
	ID        int       `gorm:"primarykey;autoIncrement" json:"id"` // 节点ID
	CreatedAt time.Time `json:"created_at"`                         // 创建时间
	UpdatedAt time.Time `json:"updated_at"`                         // 更新时间
	Name      string    `gorm:"size:255" json:"name"`               // 节点名称
	Token     string    `gorm:"size:255" json:"token"`              // 认证令牌

	// 网络配置
	IPv4       string `gorm:"size:45" json:"ipv4"`         // IPv4地址
	IPv6       string `gorm:"size:45" json:"ipv6"`         // IPv6地址
	Peers      string `gorm:"type:text" json:"peers"`      // 对等节点列表(JSON)
	Endpoints  string `gorm:"type:text" json:"endpoints"`  // 可访问的端点(JSON)
	PublicKey  string `gorm:"size:255" json:"public_key"`  // WireGuard公钥
	PrivateKey string `gorm:"size:255" json:"private_key"` // WireGuard私钥

	// 服务配置
	WireGuard string `json:"wireguard"` // WireGuard配置(JSON)
	Babel     string `json:"babel"`     // Babeld配置

	// 网络参数
	MTU           int    `json:"mtu"`                           // MTU大小
	BasePort      int    `json:"base_port"`                     // 基础端口
	LinkLocalNet  string `gorm:"size:45" json:"link_local_net"` // 链路本地网络
	BabelPort     int    `json:"babel_port"`                    // Babeld端口
	BabelInterval int    `json:"babel_interval"`                // Babeld更新间隔

	Status NodeStatus `gorm:"foreignKey:ID" json:"status"`
}

// NodeStatus 节点状态
type NodeStatus struct {
	ID           int           `gorm:"primarykey" json:"id"`
	NodeID       int           `gorm:"index" json:"node_id"`
	Hostname     string        `gorm:"type:varchar(255)" json:"hostname"`
	IPAddress    string        `gorm:"type:varchar(255)" json:"ip_address"`
	Metrics      SystemMetrics `gorm:"embedded" json:"metrics"`
	RunningTasks []string      `gorm:"type:json" json:"running_tasks"`
	Status       string        `gorm:"type:varchar(50)" json:"status"`
	Version      string        `gorm:"type:varchar(50)" json:"version"`
	Timestamp    time.Time     `gorm:"autoUpdateTime" json:"timestamp"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage    float64 `gorm:"type:decimal(5,2)" json:"cpu_usage"`
	MemoryUsage float64 `gorm:"type:decimal(5,2)" json:"memory_usage"`
	DiskUsage   float64 `gorm:"type:decimal(5,2)" json:"disk_usage"`
	Uptime      int64   `gorm:"type:bigint" json:"uptime"`
}
