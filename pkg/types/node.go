package types

import "time"

// NodeConfig 节点配置
type NodeConfig struct {
	// 基本信息
	ID        int       `json:"id"`         // 节点ID
	Name      string    `json:"name"`       // 节点名称
	Token     string    `json:"token"`      // 认证令牌
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间

	// 网络配置
	IPv4       string   `json:"ipv4"`        // IPv4地址
	IPv6       string   `json:"ipv6"`        // IPv6地址
	Peers      []string `json:"peers"`       // 对等节点列表
	Endpoints  []string `json:"endpoints"`   // 可访问的端点
	PublicKey  string   `json:"public_key"`  // WireGuard公钥
	PrivateKey string   `json:"private_key"` // WireGuard私钥

	// 服务配置
	WireGuard map[string]string `json:"wireguard"` // WireGuard配置
	Babel     string            `json:"babel"`     // Babeld配置

	// 网络参数
	Network struct {
		MTU           int    `json:"mtu"`            // MTU大小
		BasePort      int    `json:"base_port"`      // 基础端口
		LinkLocalNet  string `json:"link_local_net"` // 链路本地网络
		BabelPort     int    `json:"babel_port"`     // Babeld端口
		BabelInterval int    `json:"babel_interval"` // Babeld更新间隔
	} `json:"network"`
}

// NodeStatus 节点状态
type NodeStatus struct {
	ID            int       `json:"id"`              // 节点ID
	Name          string    `json:"name"`            // 节点名称
	Version       string    `json:"version"`         // 版本
	StartTime     time.Time `json:"start_time"`      // 启动时间
	LastSeen      time.Time `json:"last_seen"`       // 最后一次心跳时间
	LastError     string    `json:"last_error"`      // 最后一次错误
	LastErrorTime string    `json:"last_error_time"` // 最后一次错误时间
	Status        string    `json:"status"`          // 状态

	// 系统状态
	System struct {
		CPUUsage    float64 `json:"cpu_usage"`    // CPU使用率
		MemoryUsage float64 `json:"memory_usage"` // 内存使用率
		DiskUsage   float64 `json:"disk_usage"`   // 磁盘使用率
		Uptime      int64   `json:"uptime"`       // 运行时间
	} `json:"system"`

	// 网络状态
	Network struct {
		WireGuard struct {
			Status    string            `json:"status"`     // WireGuard状态
			Peers     map[string]string `json:"peers"`      // 对等节点状态
			TxBytes   int64             `json:"tx_bytes"`   // 发送字节数
			RxBytes   int64             `json:"rx_bytes"`   // 接收字节数
			LastError string            `json:"last_error"` // 最后一次错误
		} `json:"wireguard"`

		Babel struct {
			Status    string            `json:"status"`     // Babeld状态
			Routes    int               `json:"routes"`     // 路由数量
			Neighbors map[string]string `json:"neighbors"`  // 邻居节点状态
			LastError string            `json:"last_error"` // 最后一次错误
		} `json:"babel"`
	} `json:"network"`
}
