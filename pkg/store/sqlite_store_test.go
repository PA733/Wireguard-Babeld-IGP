package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mesh-backend/pkg/types"
)

func TestSQLiteStore(t *testing.T) {
	// 创建临时数据库文件
	dbFile := "test.db"
	defer os.Remove(dbFile)

	// 初始化存储
	config := DefaultSQLiteConfig(dbFile)
	store, err := NewSQLiteStore(config)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// 测试节点操作
	t.Run("Node Operations", func(t *testing.T) {
		// 创建测试节点
		node := &types.NodeConfig{
			WireGuard: map[string]string{
				"peer1": "private-key=xxx\npublic-key=yyy\n",
			},
			Babel: "protocol babel\nredistribute ipv4 ::/0 le 128\n",
		}
		node.NodeInfo.ID = 1
		node.NodeInfo.Name = "test-node"
		node.NodeInfo.IPv4 = "10.0.0.1"
		node.NodeInfo.IPv6 = "fd00::1"
		node.NodeInfo.Peers = []string{"peer1", "peer2"}
		node.NodeInfo.Endpoints = []string{"endpoint1", "endpoint2"}
		node.NodeInfo.PublicKey = "public-key-1"
		node.NodeInfo.PrivateKey = "private-key-1"
		node.Network.MTU = 1420
		node.Network.BasePort = 36420
		node.Network.LinkLocalNet = "fe80::/64"
		node.Network.BabelPort = 6696
		node.Network.BabelInterval = 5000

		// 测试创建节点
		err := store.CreateNode(ctx, node)
		assert.NoError(t, err)

		// 测试获取节点
		retrieved, err := store.GetNode(ctx, node.NodeInfo.ID)
		assert.NoError(t, err)
		assert.Equal(t, node.NodeInfo.Name, retrieved.NodeInfo.Name)
		assert.Equal(t, node.NodeInfo.IPv4, retrieved.NodeInfo.IPv4)
		assert.Equal(t, node.NodeInfo.IPv6, retrieved.NodeInfo.IPv6)
		assert.Equal(t, node.NodeInfo.PublicKey, retrieved.NodeInfo.PublicKey)
		assert.Equal(t, node.NodeInfo.PrivateKey, retrieved.NodeInfo.PrivateKey)
		assert.Equal(t, node.Network.MTU, retrieved.Network.MTU)
		assert.Equal(t, node.Network.BabelPort, retrieved.Network.BabelPort)
	})

	// 测试节点状态操作
	t.Run("Node Status Operations", func(t *testing.T) {
		nodeID := 1
		status := &types.NodeStatus{
			ID:        nodeID,
			Name:      "test-node",
			Version:   "1.0.0",
			StartTime: time.Now().Format(time.RFC3339),
		}
		status.System.CPUUsage = 50.0
		status.System.MemoryUsage = 30.0
		status.System.DiskUsage = 40.0
		status.System.Uptime = 3600

		status.Network.WireGuard.Status = "running"
		status.Network.WireGuard.Peers = map[string]string{
			"peer1": "connected",
			"peer2": "connected",
		}
		status.Network.WireGuard.TxBytes = 1000
		status.Network.WireGuard.RxBytes = 2000

		status.Network.Babel.Status = "running"
		status.Network.Babel.Routes = 10
		status.Network.Babel.Neighbors = map[string]string{
			"peer1": "reachable",
			"peer2": "reachable",
		}

		// 测试更新状态
		err := store.UpdateNodeStatus(ctx, nodeID, status)
		assert.NoError(t, err)

		// 测试获取状态
		retrieved, err := store.GetNodeStatus(ctx, nodeID)
		assert.NoError(t, err)
		assert.Equal(t, status.Name, retrieved.Name)
		assert.Equal(t, status.Version, retrieved.Version)
		assert.Equal(t, status.System.CPUUsage, retrieved.System.CPUUsage)
		assert.Equal(t, status.System.MemoryUsage, retrieved.System.MemoryUsage)
		assert.Equal(t, status.Network.WireGuard.Status, retrieved.Network.WireGuard.Status)
		assert.Equal(t, status.Network.Babel.Status, retrieved.Network.Babel.Status)
	})
}
