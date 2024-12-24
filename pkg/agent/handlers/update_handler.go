package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
)

// UpdateHandler 处理配置更新任务
type UpdateHandler struct {
	config *config.AgentConfig
	logger zerolog.Logger
}

// NewUpdateHandler 创建配置更新处理器
func NewUpdateHandler(cfg *config.AgentConfig, logger zerolog.Logger) *UpdateHandler {
	return &UpdateHandler{
		config: cfg,
		logger: logger.With().Str("handler", "update").Logger(),
	}
}

// CanHandle 检查是否可以处理该类型的任务
func (h *UpdateHandler) CanHandle(taskType types.TaskType) bool {
	return taskType == types.TaskTypeUpdate
}

// Handle 处理配置更新任务
func (h *UpdateHandler) Handle(task *types.Task) (*types.TaskResult, error) {
	logger := h.logger.With().Str("task_id", task.ID).Logger()
	logger.Info().Msg("Starting configuration update")

	// 获取最新配置
	config, err := h.fetchConfig()
	if err != nil {
		detailBytes, _ := json.Marshal(map[string]interface{}{"error": err.Error()})
		return &types.TaskResult{
			Status:    types.TaskStatusFailed,
			Error:     fmt.Sprintf("fetching config: %v", err),
			Details:   string(detailBytes),
			Timestamp: time.Now(),
		}, err
	}

	// 如果是dry-run模式，只记录不实际更新
	if h.config.Runtime.DryRun {
		logger.Info().Msg("Dry run mode, skipping actual update")
		detailBytes, _ := json.Marshal(map[string]interface{}{"config": config})
		return &types.TaskResult{
			Status:    types.TaskStatusSuccess,
			Details:   string(detailBytes),
			Timestamp: time.Now(),
		}, nil
	}

	// 更新WireGuard配置
	var configs map[string]string
	err = json.Unmarshal([]byte(config.WireGuard), &configs)
	if err != nil {
		log.Fatal(err)
	}
	if err := h.updateWireGuardConfig(configs); err != nil {
		detailBytes, _ := json.Marshal(map[string]interface{}{"error": err.Error()})
		return &types.TaskResult{
			Status:    types.TaskStatusFailed,
			Error:     fmt.Sprintf("updating wireguard config: %v", err),
			Details:   string(detailBytes),
			Timestamp: time.Now(),
		}, err
	}

	// 更新Babeld配置
	if err := h.updateBabeldConfig(config.Babel); err != nil {
		detailBytes, _ := json.Marshal(map[string]interface{}{"error": err.Error()})
		return &types.TaskResult{
			Status:    types.TaskStatusFailed,
			Error:     fmt.Sprintf("updating babeld config: %v", err),
			Details:   string(detailBytes),
			Timestamp: time.Now(),
		}, err
	}

	// 重启服务
	// if err := h.restartServices(); err != nil {
	// 	return &types.TaskResult{
	// 		Status:    types.TaskStatusFailed,
	// 		Error:     fmt.Sprintf("restarting services: %v", err),
	// 		Details:   map[string]interface{}{"error": err.Error()},
	// 		Timestamp: time.Now(),
	// 	}, err
	// }

	detailBytes, _ := json.Marshal(map[string]interface{}{"message": "Configuration updated successfully"})
	return &types.TaskResult{
		Status:    types.TaskStatusSuccess,
		Details:   string(detailBytes),
		Timestamp: time.Now(),
	}, nil
}

// fetchConfig 从服务器获取最新配置
func (h *UpdateHandler) fetchConfig() (*types.NodeConfig, error) {
	url := fmt.Sprintf("%s/nodes/%d/config", h.config.Server.Address, h.config.NodeID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var config types.NodeConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// updateWireGuardConfig 更新WireGuard配置
func (h *UpdateHandler) updateWireGuardConfig(configs map[string]string) error {
	// 确保配置目录存在
	configDir := filepath.Dir(h.config.WireGuard.ConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// 获取基础文件名（不包含扩展名）
	baseFileName := strings.TrimSuffix(h.config.WireGuard.ConfigPath, filepath.Ext(h.config.WireGuard.ConfigPath))

	// 删除旧的配置文件
	oldFiles, err := filepath.Glob(baseFileName + "-*.conf")
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to list old config files")
	} else {
		for _, file := range oldFiles {
			if err := os.Remove(file); err != nil {
				h.logger.Warn().Err(err).Str("file", file).Msg("Failed to remove old config file")
			}
		}
	}

	// 写入新的配置文件
	for peerID, config := range configs {
		configPath := fmt.Sprintf("%s-%s.conf", baseFileName, peerID)
		if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
			return fmt.Errorf("writing config file %s: %w", configPath, err)
		}
		h.logger.Debug().
			Str("path", configPath).
			Str("peer", peerID).
			Msg("WireGuard config updated")
	}

	return nil
}

// updateBabeldConfig 更新Babeld配置
func (h *UpdateHandler) updateBabeldConfig(config string) error {
	// 确保配置目录存在
	if err := os.MkdirAll(filepath.Dir(h.config.Babel.ConfigPath), 0755); err != nil {
		return err
	}

	// 写入新配置
	if err := os.WriteFile(h.config.Babel.ConfigPath, []byte(config), 0600); err != nil {
		return err
	}

	return nil
}

// restartServices 重启网络服务
// func (h *UpdateHandler) restartServices() error {
// 	// 重启WireGuard接口
// 	if err := h.restartWireGuard(); err != nil {
// 		return fmt.Errorf("restarting wireguard: %w", err)
// 	}

// 	// 重启Babeld服务
// 	if err := h.restartBabeld(); err != nil {
// 		return fmt.Errorf("restarting babeld: %w", err)
// 	}

// 	return nil
// }

// restartWireGuard 重启WireGuard接口
// func (h *UpdateHandler) restartWireGuard() error {
// 	// 获取配置文件基础名
// 	baseFileName := strings.TrimSuffix(h.config.WireGuard.ConfigPath, filepath.Ext(h.config.WireGuard.ConfigPath))

// 	// 关闭所有WireGuard接口
// 	cmd := exec.Command(h.config.WireGuard.BinPath, "down", "wg*")
// 	if err := cmd.Run(); err != nil {
// 		h.logger.Warn().Err(err).Msg("Failed to down WireGuard interfaces")
// 	}

// 	// 启动所有WireGuard接口
// 	configs, err := filepath.Glob(baseFileName + "-*.conf")
// 	if err != nil {
// 		return fmt.Errorf("listing config files: %w", err)
// 	}

// 	for _, config := range configs {
// 		cmd = exec.Command(h.config.WireGuard.BinPath, "up", "-f", config)
// 		if err := cmd.Run(); err != nil {
// 			return fmt.Errorf("starting interface with config %s: %w", config, err)
// 		}
// 		h.logger.Info().Str("config", config).Msg("WireGuard interface started")
// 	}

// 	return nil
// }

// restartBabeld 重启Babeld服务
// func (h *UpdateHandler) restartBabeld() error {
// 	// 停止Babeld服务
// 	cmd := exec.Command("systemctl", "stop", "babeld")
// 	if err := cmd.Run(); err != nil {
// 		h.logger.Warn().Err(err).Msg("Failed to stop babeld service")
// 	}

// 	// 启动Babeld服务
// 	cmd = exec.Command("systemctl", "start", "babeld")
// 	return cmd.Run()
// }
