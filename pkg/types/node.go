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
	ID            int       `gorm:"primarykey;autoIncrement" json:"id"` // 节点ID
	CreatedAt     time.Time `json:"created_at"`                         // 创建时间
	UpdatedAt     time.Time `json:"updated_at"`                         // 更新时间
	Name          string    `gorm:"size:255" json:"name"`               // 节点名称
	Version       string    `gorm:"size:50" json:"version"`             // 版本
	StartTime     time.Time `json:"start_time"`                         // 启动时间
	LastSeen      time.Time `json:"last_seen"`                          // 最后一次心跳时间
	LastError     string    `gorm:"type:text" json:"last_error"`        // 最后一次错误
	LastErrorTime string    `json:"last_error_time"`                    // 最后一次错误时间
	Status        string    `gorm:"size:50" json:"status"`              // 状态

	// 系统状态
	CPUUsage    float64 `json:"cpu_usage"`    // CPU使用率
	MemoryUsage float64 `json:"memory_usage"` // 内存使用率
	DiskUsage   float64 `json:"disk_usage"`   // 磁盘使用率
	Uptime      int64   `json:"uptime"`       // 运行时间

	// 网络状态(JSON格式存储)
	WireGuardStatus string `gorm:"type:text" json:"wireguard_status"` // WireGuard状态
	BabelStatus     string `gorm:"type:text" json:"babel_status"`     // Babel状态
}
