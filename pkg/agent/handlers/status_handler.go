package handlers

import (
	"time"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
)

// StatusHandler 处理状态报告任务
type StatusHandler struct {
	config *config.AgentConfig
	logger zerolog.Logger
}

// NewStatusHandler 创建状态报告处理器
func NewStatusHandler(cfg *config.AgentConfig, logger zerolog.Logger) *StatusHandler {
	return &StatusHandler{
		config: cfg,
		logger: logger.With().Str("handler", "status").Logger(),
	}
}

// CanHandle 检查是否可以处理该类型的任务
func (h *StatusHandler) CanHandle(taskType types.TaskType) bool {
	return taskType == types.TaskTypeStatus
}

// Handle 处理状态报告任务
func (h *StatusHandler) Handle(task *types.Task) (*types.TaskResult, error) {
	logger := h.logger.With().Str("task_id", task.ID).Logger()
	logger.Info().Msg("Starting status report")

	details := make(map[string]interface{})

	// 检查WireGuard状态
	// if wgStatus, err := h.checkWireGuardStatus(); err != nil {
	// 	logger.Warn().Err(err).Msg("Failed to check WireGuard status")
	// 	details["wireguard"] = map[string]interface{}{
	// 		"status": "error",
	// 		"error":  err.Error(),
	// 	}
	// } else {
	// 	details["wireguard"] = wgStatus
	// }

	// 检查Babeld状态
	// if babelStatus, err := h.checkBabeldStatus(); err != nil {
	// 	logger.Warn().Err(err).Msg("Failed to check Babeld status")
	// 	details["babel"] = map[string]interface{}{
	// 		"status": "error",
	// 		"error":  err.Error(),
	// 	}
	// } else {
	// 	details["babel"] = babelStatus
	// }

	return &types.TaskResult{
		Status:    types.TaskStatusSuccess,
		Details:   details,
		Error:     "",
		Timestamp: time.Now(),
	}, nil
}

// checkWireGuardStatus 检查WireGuard状态
// func (h *StatusHandler) checkWireGuardStatus() (map[string]interface{}, error) {
// 	cmd := exec.Command(h.config.WireGuard.BinPath, "show")
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return nil, fmt.Errorf("executing wg show: %w", err)
// 	}

// 	return map[string]interface{}{
// 		"status":  "running",
// 		"output":  string(output),
// 		"updated": time.Now(),
// 	}, nil
// }

// checkBabeldStatus 检查Babeld状态
// func (h *StatusHandler) checkBabeldStatus() (map[string]interface{}, error) {
// 	cmd := exec.Command("systemctl", "status", "babeld")
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return nil, fmt.Errorf("checking babeld service: %w", err)
// 	}

// 	return map[string]interface{}{
// 		"status":  "running",
// 		"output":  string(output),
// 		"updated": time.Now(),
// 	}, nil
// }
