package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"mesh-backend/pkg/store"
	"net/http"

	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// NodeAuthenticator 实现节点认证
type NodeAuthenticator struct {
	logger zerolog.Logger
	store  store.Store
}

// NewNodeAuthenticator 创建节点认证器
func NewNodeAuthenticator(logger zerolog.Logger, store store.Store) *NodeAuthenticator {
	return &NodeAuthenticator{
		logger: logger.With().Str("component", "node_auth").Logger(),
		store:  store,
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

// ValidateToken 验证节点令牌
func (a *NodeAuthenticator) ValidateToken(nodeID int, token string) bool {
	node, err := a.store.GetNode(nodeID)
	if err != nil {
		a.logger.Debug().
			Int("node_id", nodeID).
			Bool("token_exists", false).
			Bool("token_valid", false).
			Msg("Validating node token")
		return false
	}
	if token != node.Token {
		a.logger.Debug().
			Int("node_id", nodeID).
			Bool("token_exists", true).
			Bool("token_valid", false).
			Msg("Validating node token")
		return false
	}
	a.logger.Debug().
		Int("node_id", nodeID).
		Bool("token_exists", true).
		Bool("token_valid", true).
		Msg("Validating node token")
	return true
}

// NodeAuth 节点认证中间件
func (a *NodeAuthenticator) NodeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID, token, ok := c.Request.BasicAuth()
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Basic authentication is required"})
			c.Abort()
			return
		}
		nodeIDInt, err := strconv.Atoi(nodeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
			c.Abort()
			return
		}
		if !a.ValidateToken(nodeIDInt, token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid node token"})
			c.Abort()
			return
		}
		c.Next()
	}
}
