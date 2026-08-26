package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"time"

	cpconfig "RedisShake/internal/controlplane/config"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/engine"
	"RedisShake/internal/controlplane/store"
	"RedisShake/internal/controlplane/tasks"
	"RedisShake/internal/controlplane/webassets"
)

type BuildInfo struct {
	Version   string
	GitCommit string
}

type Server struct {
	store       *store.Store
	config      cpconfig.Config
	buildInfo   BuildInfo
	connections *connections.Service
	tasks       *tasks.Service
	engine      *engine.Manager
	startedAt   time.Time
	webFS       fs.FS
	handler     http.Handler
}

func NewServer(database *store.Store, config cpconfig.Config, buildInfo BuildInfo, connectionService *connections.Service, taskService *tasks.Service, engineManager *engine.Manager) *Server {
	server := &Server{
		store:       database,
		config:      config,
		buildInfo:   buildInfo,
		connections: connectionService,
		tasks:       taskService,
		engine:      engineManager,
		startedAt:   time.Now().UTC(),
	}
	if config.WebDir != "" {
		server.webFS = os.DirFS(config.WebDir)
	} else if embedded, available := webassets.FileSystem(); available {
		server.webFS = embedded
	}
	server.handler = server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleReady)
	mux.HandleFunc("GET /api/v1/system/info", s.handleSystemInfo)
	mux.HandleFunc("GET /api/v1/connections", s.handleListConnections)
	mux.HandleFunc("POST /api/v1/connections", s.handleCreateConnection)
	mux.HandleFunc("GET /api/v1/connections/{id}", s.handleGetConnection)
	mux.HandleFunc("PATCH /api/v1/connections/{id}", s.handleUpdateConnection)
	mux.HandleFunc("DELETE /api/v1/connections/{id}", s.handleDeleteConnection)
	mux.HandleFunc("POST /api/v1/connections/test", s.handleTestUnsavedConnection)
	mux.HandleFunc("POST /api/v1/connections/{id}/test", s.handleTestSavedConnection)
	mux.HandleFunc("POST /api/v1/connections/{id}/copy", s.handleCopyConnection)
	mux.HandleFunc("GET /api/v1/tasks", s.handleListTasks)
	mux.HandleFunc("POST /api/v1/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/v1/tasks/{id}", s.handleGetTask)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", s.handleUpdateTask)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", s.handleArchiveTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/precheck", s.handlePrecheckTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/copy", s.handleCopyTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/runs", s.handleStartRun)
	mux.HandleFunc("GET /api/v1/tasks/{id}/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/v1/runs/{id}", s.handleGetRun)
	mux.HandleFunc("POST /api/v1/runs/{id}/stop", s.handleStopRun)
	mux.HandleFunc("POST /api/v1/runs/{id}/force-stop", s.handleForceStopRun)
	mux.HandleFunc("GET /api/v1/runs/{id}/logs", s.handleRunLogs)
	mux.HandleFunc("GET /api/v1/runs/{id}/events", s.handleRunEvents)
	mux.Handle("/", s.fallbackHandler())
	return securityHeaders(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"started_at": s.startedAt,
	})
}

func (s *Server) handleReady(w http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "SQLite storage is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ready",
		"database": "ok",
	})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":             s.buildInfo.Version,
		"git_commit":          s.buildInfo.GitCommit,
		"storage":             "sqlite",
		"data_dir":            s.config.DataDir,
		"runtime_dir":         s.config.RuntimeDir,
		"secrets_configured":  s.config.SecretsConfigured(),
		"worker_path":         s.config.WorkerPath,
		"web_ui_configured":   s.webFS != nil,
		"max_concurrent_runs": s.config.MaxConcurrentRuns,
		"log_retention_days":  s.config.LogRetentionDays,
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "Resource not found")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, request)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
