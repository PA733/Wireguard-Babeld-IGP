package handlers

import (
	"encoding/json"
	"net/http"

	"mesh-backend/internal/service"
	"mesh-backend/pkg/logger"

	"github.com/rs/zerolog"
)

type NodeHandler struct {
	nodeService *service.NodeService
	log         zerolog.Logger
}

func NewNodeHandler(
	nodeService *service.NodeService,
	logger *logger.Logger,
) *NodeHandler {
	return &NodeHandler{
		nodeService: nodeService,
		log:         logger.GetLogger("node-handler"),
	}
}

type createNodeRequest struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	var req createNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.log.Debug().
		Str("name", req.Name).
		Str("endpoint", req.Endpoint).
		Msg("Creating new node")

	node, err := h.nodeService.CreateNode(req.Name, req.Endpoint)
	if err != nil {
		h.log.Error().Err(err).
			Str("name", req.Name).
			Str("endpoint", req.Endpoint).
			Msg("Failed to create node")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.log.Info().
		Int("id", node.ID).
		Str("name", node.Name).
		Str("ipv4_prefix", node.IPv4Prefix).
		Str("ipv6_prefix", node.IPv6Prefix).
		Msg("Node created successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

func (h *NodeHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.nodeService.ListNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}
