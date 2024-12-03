package api

import (
	"net/http"

	"mesh-backend/internal/api/handlers"
	"mesh-backend/pkg/logger"
)

func NewRouter(
	nodeHandler *handlers.NodeHandler,
	taskHandler *handlers.TaskHandler,
	configHandler *handlers.ConfigHandler,
	statusHandler *handlers.StatusHandler,
	logger *logger.Logger,
) http.Handler {
	log := logger.GetLogger("router")

	mux := http.NewServeMux()

	// 节点管理路由
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			nodeHandler.CreateNode(w, r)
		case http.MethodGet:
			nodeHandler.ListNodes(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 任务管理路由
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			taskHandler.ListTasks(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 配置管理路由
	mux.HandleFunc("/nodes/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nodes" {
			return
		}
		if r.Method == http.MethodGet {
			configHandler.GetNodeConfig(w, r)
		}
	})

	mux.HandleFunc("/config/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			configHandler.UpdateConfigs(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 状态管理路由
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			statusHandler.GetSystemStatus(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Debug().Msg("Router initialized")
	return mux
}
