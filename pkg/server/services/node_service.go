package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/store"
	"mesh-backend/pkg/types"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/curve25519"
)

type NodeService struct {
	config *config.ServerConfig
	logger zerolog.Logger
	store  store.Store

	// 节点管理
	nodeAuth *NodeAuthenticator
	nodes    map[int]*types.NodeConfig
	lastID   int32

	// 服务依赖
	taskService *TaskService
}

// NewNodeService 创建节点服务实例
func NewNodeService(cfg *config.ServerConfig, logger zerolog.Logger, store store.Store, taskService *TaskService, nodeAuth *NodeAuthenticator) *NodeService {
	srv := &NodeService{
		config:      cfg,
		logger:      logger.With().Str("service", "node").Logger(),
		store:       store,
		nodeAuth:    nodeAuth,
		nodes:       make(map[int]*types.NodeConfig),
		taskService: taskService,
	}

	// 初始化最大节点ID并加载 tokens
	if nodes, err := store.ListNodes(); err == nil {
		maxID := int32(0)
		for _, node := range nodes {
			if int32(node.ID) > maxID {
				maxID = int32(node.ID)
			}
			// 从数据库加载 token 到 nodeAuth
			srv.nodeAuth.RegisterNode(node.ID, node.Token)
			srv.logger.Debug().
				Int("node_id", node.ID).
				Str("token", node.Token).
				Msg("Loaded node token from database")
		}
		atomic.StoreInt32(&srv.lastID, maxID)
	}

	return srv
}

func (s *NodeService) RegisterRoutes(r *gin.Engine) {
	r.GET("/nodes", s.HandleListNodes)
	r.POST("/nodes", s.HandleCreateNode)
	r.GET("/nodes/:id", s.HandleGetNode)
	r.POST("/nodes/config/:id", s.HandleTriggerConfigUpdate)
}

func (s *NodeService) HandleListNodes(c *gin.Context) {
	nodes, err := s.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodes)
}

func (s *NodeService) HandleCreateNode(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Endpoint string `json:"endpoint" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 生成节点配置
	nodeID := s.nextNodeID()
	now := time.Now()

	token, err := s.GenerateNodeToken(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	peersBytes, _ := json.Marshal([]string{})
	endpointBytes, _ := json.Marshal([]string{req.Endpoint})
	config := &types.NodeConfig{
		// 基本信息
		ID:        nodeID,
		Name:      req.Name,
		Token:     token,
		Peers:     string(peersBytes), // To-Do 添加预设节点
		Endpoints: string(endpointBytes),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 生成 WireGuard 密钥对
	privateKey, publicKey, err := generateWireGuardKeyPair()
	if err != nil {
		// http.Error(w, err.Error(), http.StatusInternalServerError)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	config.PrivateKey = privateKey
	config.PublicKey = publicKey

	// 创建节点
	if err := s.store.CreateNode(config); err != nil {
		// http.Error(w, err.Error(), http.StatusInternalServerError)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 注册节点令牌
	s.nodeAuth.RegisterNode(config.ID, token)

	c.JSON(http.StatusOK, gin.H{
		"id":         config.ID,
		"name":       config.Name,
		"token":      token,
		"public_key": config.PublicKey,
	})
}

// nextNodeID 生成下一个节点ID
func (s *NodeService) nextNodeID() int {
	return int(atomic.AddInt32(&s.lastID, 1))
}

func (s *NodeService) HandleGetNode(c *gin.Context) {
	nodeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	node, err := s.GetNode(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	c.JSON(http.StatusOK, node)
}

func (s *NodeService) HandleTriggerConfigUpdate(c *gin.Context) {
	nodeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	if err := s.TriggerConfigUpdate(nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

// GetNode 获取节点配置
func (s *NodeService) GetNode(nodeID int) (*types.NodeConfig, error) {
	return s.store.GetNode(nodeID)
}

// ListNodes 列出所有节点
func (s *NodeService) ListNodes() ([]*types.NodeConfig, error) {
	return s.store.ListNodes()
}

// UpdateNode 更新节点配置
func (s *NodeService) UpdateNode(nodeID int, config *types.NodeConfig) error {
	// 获取原有节点配置
	_, err := s.store.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("get old node: %w", err)
	}

	// 保留原有 token
	// config.Token = oldNode.Token

	// 更新节点配置
	if err := s.store.UpdateNode(nodeID, config); err != nil {
		return fmt.Errorf("update node: %w", err)
	}

	// 确保 token 在 nodeAuth 中注册
	// s.nodeAuth.RegisterNode(nodeID, config.Token)

	return nil
}

// DeleteNode 删除节点
func (s *NodeService) DeleteNode(nodeID int) error {
	s.nodeAuth.RemoveNode(nodeID)
	return s.store.DeleteNode(nodeID)
}

// TriggerConfigUpdate 触发节点配置更新任务
func (s *NodeService) TriggerConfigUpdate(nodeID int) error {
	// task := &types.Task{
	// 	ID:        fmt.Sprintf("config_update_%d_%d", nodeID, time.Now().Unix()),
	// 	Type:      "config_update",
	// 	Status:    "pending",
	// 	CreatedAt: time.Now(),
	// 	UpdatedAt: time.Now(),
	// 	NodeID:    nodeID,
	// }

	// 保存任务
	task, err := s.taskService.CreateTask(types.TaskTypeUpdate, nodeID)
	if err != nil {
		return fmt.Errorf("creating update task: %w", err)
	}

	if err := s.taskService.PushTask(task); err != nil {
		return fmt.Errorf("saving task: %w", err)
	}

	s.logger.Info().
		Int("node_id", nodeID).
		Str("task_id", task.ID).
		Msg("Triggered config update task")

	return nil
}

// generateWireGuardKeyPair 生成WireGuard密钥对
func generateWireGuardKeyPair() (privateKey, publicKey string, err error) {
	var private, public [32]byte

	// 生成私钥
	if _, err := rand.Read(private[:]); err != nil {
		return "", "", fmt.Errorf("generating private key: %w", err)
	}

	// 生成公钥
	curve25519.ScalarBaseMult(&public, &private)

	// 编码为Base64
	privateKey = base64.StdEncoding.EncodeToString(private[:])
	publicKey = base64.StdEncoding.EncodeToString(public[:])

	return privateKey, publicKey, nil
}

// GenerateNodeToken 生成节点认证令牌
func (s *NodeService) GenerateNodeToken(nodeID int) (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}

	tokenStr := base64.URLEncoding.EncodeToString(token)
	s.nodeAuth.RegisterNode(nodeID, tokenStr)

	s.logger.Debug().
		Int("node_id", nodeID).
		Str("token", tokenStr).
		Msg("Generated and registered new token")

	return tokenStr, nil
}
