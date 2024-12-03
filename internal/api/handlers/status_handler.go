package handlers

import (
	"encoding/json"
	"net/http"

	"mesh-backend/internal/service"
	"mesh-backend/pkg/logger"

	"github.com/rs/zerolog"
)

type StatusHandler struct {
	statusService *service.StatusService
	log           zerolog.Logger
}

func NewStatusHandler(
	statusService *service.StatusService,
	logger *logger.Logger,
) *StatusHandler {
	return &StatusHandler{
		statusService: statusService,
		log:           logger.GetLogger("status-handler"),
	}
}

func (h *StatusHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	h.log.Debug().Msg("Retrieving system status")

	status, err := h.statusService.GetSystemStatus()
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to get system status")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.log.Info().
		Int("total_nodes", status.TotalNodes).
		Int("online_nodes", status.OnlineNodes).
		Int("pending_tasks", status.PendingTasks).
		Msg("System status retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
