package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pb "mesh-backend/api/proto/task"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
)

// TaskHandler 处理所有任务相关的逻辑
type TaskHandler struct {
	config *config.AgentConfig
	logger zerolog.Logger
	client pb.TaskServiceClient

	// 任务处理
	taskCh chan *pb.Task
	ctx    context.Context
}

// NewTaskHandler 创建新的任务处理器
func NewTaskHandler(cfg *config.AgentConfig, logger zerolog.Logger, client pb.TaskServiceClient, ctx context.Context) *TaskHandler {
	return &TaskHandler{
		config: cfg,
		logger: logger,
		client: client,
		taskCh: make(chan *pb.Task, 100),
		ctx:    ctx,
	}
}

// Start 启动任务处理循环
func (h *TaskHandler) Start() {
	go h.processTasksLoop()
}

// EnqueueTask 将任务加入处理队列
func (h *TaskHandler) EnqueueTask(task *pb.Task) {
	h.taskCh <- task
}

// processTasksLoop 处理任务循环
func (h *TaskHandler) processTasksLoop() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case task := <-h.taskCh:
			go h.HandleTask(task)
		}
	}
}

// HandleTask 处理单个任务
func (h *TaskHandler) HandleTask(task *pb.Task) {
	h.logger.Info().
		Str("task_id", task.Id).
		Str("type", task.Type).
		Msg("Processing task")

	var err error
	switch task.Type {
	case string(types.TaskTypeUpdate):
		err = h.handleConfigUpdate(task)
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	if err != nil {
		h.logger.Error().Err(err).
			Str("task_id", task.Id).
			Msg("Failed to process task")
		h.updateTaskStatus(task, &types.TaskResult{
			Status: types.TaskStatusFailed,
			Error:  err.Error(),
		})
	}
}

// handleConfigUpdate 处理配置更新任务
func (h *TaskHandler) handleConfigUpdate(task *pb.Task) error {
	// 获取最新配置
	url := fmt.Sprintf("%s/api/agent/config/%d", h.config.Server.Address, h.config.NodeID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("fetching config: %w", err)
	}

	auth := fmt.Sprintf("%d:%s", h.config.NodeID, h.config.Token)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Add("Authorization", "Basic "+encodedAuth)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败:%s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var config types.NodeConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return fmt.Errorf("decoding config: %w", err)
	}

	// 更新 WireGuard 配置
	var configs map[string]string
	err = json.Unmarshal([]byte(config.WireGuard), &configs)
	if err != nil {
		log.Fatal(err)
	}
	if err := h.updateWireGuardConfig(configs); err != nil {
		return fmt.Errorf("updating wireguard config: %w", err)
	}

	// 更新 Babeld 配置
	if err := h.updateBabeldConfig(config.Babel); err != nil {
		return fmt.Errorf("updating babeld config: %w", err)
	}

	h.updateTaskStatus(task, &types.TaskResult{
		Status: types.TaskStatusSuccess,
	})
	h.logger.Info().Msg("Configuration updated successfully")
	return nil
}

// updateWireGuardConfig 更新 WireGuard 配置
func (h *TaskHandler) updateWireGuardConfig(configs map[string]string) error {
	for peerName, config := range configs {
		configPath := filepath.Join(h.config.WireGuard.ConfigPath, fmt.Sprintf("%s%s.conf", h.config.WireGuard.Prefix, peerName))
		if !h.config.Runtime.DryRun {
			if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
				return fmt.Errorf("writing wireguard config: %w", err)
			}
		} else {
			h.logger.Info().Str("DryRun", "wireguard_config").Str("path", configPath).Msg("Would run: " + config)
		}

		// 重启 WireGuard 接口
		if err := h.restartWireGuard(fmt.Sprintf("%s%s", h.config.WireGuard.Prefix, peerName)); err != nil {
			return fmt.Errorf("restarting wireguard: %w", err)
		}
	}
	return nil
}

// restartWireGuard 重启 WireGuard
func (h *TaskHandler) restartWireGuard(interfaceName string) error {
	cmd := exec.Command("systemctl", "restart", "wg-quick@"+interfaceName)
	if !h.config.Runtime.DryRun {
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restarting wireguard: %w", err)
		}
	} else {
		h.logger.Info().Str("DryRun", "wireguard_interface").Msg("Would run: " + cmd.String())
	}
	return nil
}

// updateBabeldConfig 更新 Babeld 配置
func (h *TaskHandler) updateBabeldConfig(config string) error {
	config = strings.ReplaceAll(config, "{WGPrefix}", h.config.WireGuard.Prefix)
	if !h.config.Runtime.DryRun {
		if err := os.WriteFile(h.config.Babel.ConfigPath, []byte(config), 0644); err != nil {
			return fmt.Errorf("writing babeld config: %w", err)
		}
	} else {
		h.logger.Info().Str("DryRun", "babeld_config").Msg("Would run: " + config)
	}

	// 重启 Babeld 进程
	if err := h.restartBabeld(); err != nil {
		return fmt.Errorf("restarting babeld: %w", err)
	}
	return nil
}

// restartBabeld 重启 Babeld
func (h *TaskHandler) restartBabeld() error {
	cmd := exec.Command("systemctl", "restart", "babeld")
	if !h.config.Runtime.DryRun {
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restarting babeld: %w", err)
		}
	} else {
		h.logger.Info().Str("DryRun", "babeld").Msg("Would run: " + cmd.String())
	}
	return nil
}

// updateTaskStatus 更新任务状态
func (h *TaskHandler) updateTaskStatus(task *pb.Task, result *types.TaskResult) {
	req := &pb.UpdateTaskStatusRequest{
		TaskId: task.Id,
		Status: string(result.Status),
		Error:  result.Error,
	}

	_, err := h.client.UpdateTaskStatus(context.Background(), req)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("task_id", task.Id).
			Msg("Failed to update task status")
	}
}
