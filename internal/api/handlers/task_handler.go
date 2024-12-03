package handlers

import (
	"encoding/json"
	"net/http"

	"mesh-backend/internal/service"
	"mesh-backend/pkg/logger"

	"github.com/rs/zerolog"
)

type TaskHandler struct {
	taskService *service.TaskService
	log         zerolog.Logger
}

func NewTaskHandler(
	taskService *service.TaskService,
	logger *logger.Logger,
) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
		log:         logger.GetLogger("task-handler"),
	}
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	h.log.Debug().Msg("Listing all tasks")

	tasks, err := h.taskService.ListTasks()
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to list tasks")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.log.Info().Int("count", len(tasks)).Msg("Tasks retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
