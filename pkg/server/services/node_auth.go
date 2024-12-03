package services

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
)

// NodeAuthenticator 实现节点认证
type NodeAuthenticator struct {
	tokens map[int]string
	mu     sync.RWMutex
}

// NewNodeAuthenticator 创建节点认证器
func NewNodeAuthenticator() *NodeAuthenticator {
	return &NodeAuthenticator{
		tokens: make(map[int]string),
	}
}

// GenerateToken 生成新的节点令牌
func (a *NodeAuthenticator) GenerateToken() (string, error) {
	// 生成32字节的随机令牌
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(token), nil
}

// RegisterNode 注册节点令牌
func (a *NodeAuthenticator) RegisterNode(nodeID int, token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tokens[nodeID] = token
}

// ValidateToken 验证节点令牌
func (a *NodeAuthenticator) ValidateToken(nodeID int, token string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	storedToken, exists := a.tokens[nodeID]
	return exists && storedToken == token
}

// RemoveNode 移除节点令牌
func (a *NodeAuthenticator) RemoveNode(nodeID int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.tokens, nodeID)
}

// UnregisterNode 注销节点（与 RemoveNode 相同）
func (a *NodeAuthenticator) UnregisterNode(nodeID int) {
	a.RemoveNode(nodeID)
}
