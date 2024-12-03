package types

// NodeConfig 节点配置
type NodeConfig struct {
	// WireGuard配置，key为对等节点ID，value为对应的配置文件内容
	WireGuard map[string]string `json:"wireguard"`

	// Babeld配置
	Babel string `json:"babel"`

	// 节点信息
	NodeInfo struct {
		ID         int      `json:"id"`
		Name       string   `json:"name"`
		IPv4       string   `json:"ipv4"`
		IPv6       string   `json:"ipv6"`
		Peers      []string `json:"peers"`
		Endpoints  []string `json:"endpoints"`
		PublicKey  string   `json:"public_key"`  // WireGuard公钥
		PrivateKey string   `json:"private_key"` // WireGuard私钥
	} `json:"node_info"`

	// 网络配置
	Network struct {
		MTU           int    `json:"mtu"`
		BasePort      int    `json:"base_port"`
		LinkLocalNet  string `json:"link_local_net"`
		BabelPort     int    `json:"babel_port"`
		BabelInterval int    `json:"babel_interval"`
	} `json:"network"`
}

// NodeStatus 节点状态
type NodeStatus struct {
	// 基本信息
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	StartTime string `json:"start_time"`

	// 系统状态
	System struct {
		CPUUsage    float64 `json:"cpu_usage"`
		MemoryUsage float64 `json:"memory_usage"`
		DiskUsage   float64 `json:"disk_usage"`
		Uptime      int64   `json:"uptime"`
	} `json:"system"`

	// 网络状态
	Network struct {
		WireGuard struct {
			Status    string            `json:"status"`
			Peers     map[string]string `json:"peers"`
			TxBytes   int64             `json:"tx_bytes"`
			RxBytes   int64             `json:"rx_bytes"`
			LastError string            `json:"last_error"`
		} `json:"wireguard"`

		Babel struct {
			Status    string            `json:"status"`
			Routes    int               `json:"routes"`
			Neighbors map[string]string `json:"neighbors"`
			LastError string            `json:"last_error"`
		} `json:"babel"`
	} `json:"network"`

	// 错误信息
	LastError     string `json:"last_error"`
	LastErrorTime string `json:"last_error_time"`
}
