package models

import (
	"fmt"
	"time"
)

type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
)

type Node struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	IPv4Prefix    string     `json:"ipv4_prefix"`     // 例如: "10.42.3.0"
	IPv6Prefix    string     `json:"ipv6_prefix"`     // 例如: "2a13:a5c7:21ff:276:3::"
	LinkLocalAddr string     `json:"link_local_addr"` // 例如: "fe80::3"
	Endpoint      string     `json:"endpoint"`        // 公网访问地址
	PublicKey     string     `json:"public_key"`      // WireGuard 公钥
	PrivateKey    string     `json:"-"`               // WireGuard 私钥，不输出到 JSON
	Status        NodeStatus `json:"status"`
	Token         string     `json:"token,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type NodeConfig struct {
	WireGuard map[string]string `json:"wireguard"`
	Babeld    string            `json:"babeld"`
}

func (n *Node) GetPeerIP(peerID int, isIPv6 bool) string {
	if isIPv6 {
		return fmt.Sprintf("%s%d", n.IPv6Prefix, peerID)
	}
	prefix := n.IPv4Prefix[:len(n.IPv4Prefix)-1]
	return fmt.Sprintf("%s%d", prefix, peerID)
}

func (n *Node) GetLinkLocalPeerAddr(peerID int) string {
	return fmt.Sprintf("fe80::%d:%d/64", n.ID, peerID)
}
