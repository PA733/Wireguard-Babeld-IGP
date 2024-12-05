package services

import (
	"crypto/rand"
	"encoding/base64"
	"sync"

	"github.com/rs/zerolog"
)

// NodeAuthenticator 实现节点认证
type NodeAuthenticator struct {
	tokens map[int]string
	mu     sync.RWMutex
	logger zerolog.Logger
}

// NewNodeAuthenticator 创建节点认证器
func NewNodeAuthenticator(logger zerolog.Logger) *NodeAuthenticator {
	return &NodeAuthenticator{
		tokens: make(map[int]string),
		logger: logger.With().Str("component", "node_auth").Logger(),
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
	a.logger.Debug().
		Int("node_id", nodeID).
		Str("token", token).
		Msg("Registered node token")
}

// ValidateToken 验证节点令牌
func (a *NodeAuthenticator) ValidateToken(nodeID int, token string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	storedToken, exists := a.tokens[nodeID]
	valid := exists && storedToken == token
	a.logger.Debug().
		Int("node_id", nodeID).
		Bool("token_exists", exists).
		Bool("token_valid", valid).
		Msg("Validating node token")
	return valid
}

// RemoveNode 移除节点令牌
func (a *NodeAuthenticator) RemoveNode(nodeID int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.tokens, nodeID)
	a.logger.Debug().
		Int("node_id", nodeID).
		Msg("Removed node token")
}

// UnregisterNode 注销节点（与 RemoveNode 相同）
func (a *NodeAuthenticator) UnregisterNode(nodeID int) {
	a.RemoveNode(nodeID)
}
