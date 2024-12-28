package types

import "time"

// WireguardConnection 定义Wireguard连接
type WireguardConnection struct {
	ID        int       `gorm:"primarykey" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	NodeID    int       `gorm:"index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"node_id"` // 节点ID
	PeerID    int       `gorm:"index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"peer_id"` // 对等节点ID
	Port      int       `json:"port"`                                                               // 端口

	Node NodeConfig `gorm:"foreignKey:NodeID" json:"node"` // 节点引用
	Peer NodeConfig `gorm:"foreignKey:PeerID" json:"peer"` // 对等节点引用
}
