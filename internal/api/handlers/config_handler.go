package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"mesh-backend/internal/models"
	"mesh-backend/internal/service"
	"mesh-backend/internal/store/memory"
	"mesh-backend/pkg/logger"

	"github.com/rs/zerolog"
)

type ConfigHandler struct {
	configService *service.ConfigService
	log           zerolog.Logger
}

func NewConfigHandler(
	configService *service.ConfigService,
	logger *logger.Logger,
) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
		log:           logger.GetLogger("config-handler"),
	}
}

func (h *ConfigHandler) GetNodeConfig(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		h.log.Error().Str("path", r.URL.Path).Msg("Invalid path format")
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	var nodeID int
	if _, err := fmt.Sscanf(parts[2], "%d", &nodeID); err != nil {
		h.log.Error().Err(err).Str("node_id", parts[2]).Msg("Invalid node ID format")
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	h.log.Debug().Int("node_id", nodeID).Msg("Retrieving node configuration")

	config, err := h.configService.GetNodeConfig(nodeID)
	if err != nil {
		if err == memory.ErrNodeNotFound {
			h.log.Error().Err(err).Int("node_id", nodeID).Msg("Node not found")
			http.Error(w, "Node not found", http.StatusNotFound)
		} else {
			h.log.Error().Err(err).Int("node_id", nodeID).Msg("Failed to get node config")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	h.log.Info().Int("node_id", nodeID).Msg("Node configuration retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (h *ConfigHandler) UpdateConfigs(w http.ResponseWriter, r *http.Request) {
	var req models.ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.log.Debug().
		Interface("nodes", req.Nodes).
		Msg("Updating configurations for nodes")

	if err := h.configService.UpdateConfigs(req.Nodes); err != nil {
		h.log.Error().Err(err).Interface("nodes", req.Nodes).Msg("Failed to update configurations")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.log.Info().
		Int("node_count", len(req.Nodes)).
		Msg("Configurations updated successfully")

	response := models.ConfigUpdateResponse{
		Status:  "success",
		Message: "Configurations updated and push tasks queued",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
