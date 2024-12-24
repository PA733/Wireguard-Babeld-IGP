package types

// WireguardConnection 定义Wireguard连接
type WireguardConnection struct {
	NodeID int `json:"node_id"` // 节点ID
	PeerID int `json:"peer_id"` // 对等节点ID
	Port   int `json:"port"`    // 端口
}
