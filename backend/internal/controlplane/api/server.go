package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	cpconfig "RedisShake/internal/controlplane/config"
	"RedisShake/internal/controlplane/store"
)

type BuildInfo struct {
	Version   string
	GitCommit string
}

type Server struct {
	store     *store.Store
	config    cpconfig.Config
	buildInfo BuildInfo
	startedAt time.Time
	handler   http.Handler
}

func NewServer(database *store.Store, config cpconfig.Config, buildInfo BuildInfo) *Server {
	server := &Server{
		store:     database,
		config:    config,
		buildInfo: buildInfo,
		startedAt: time.Now().UTC(),
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
	mux.HandleFunc("/", s.handleNotFound)
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
		"version":            s.buildInfo.Version,
		"git_commit":         s.buildInfo.GitCommit,
		"storage":            "sqlite",
		"data_dir":           s.config.DataDir,
		"runtime_dir":        s.config.RuntimeDir,
		"secrets_configured": s.config.SecretsConfigured(),
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "Resource not found")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
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
