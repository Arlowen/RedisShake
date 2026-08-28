package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	cpconfig "RedisShake/internal/controlplane/config"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
	"RedisShake/internal/controlplane/taskconfig"
	"RedisShake/internal/controlplane/tasks"
	"RedisShake/internal/controlplane/webassets"
)

func TestHealthAndReady(t *testing.T) {
	database, _, handler := newTestServer(t)
	defer database.Close()

	for _, path := range []string{"/healthz", "/readyz"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, body = %s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s missing no-store header", path)
		}
	}

}

func TestSystemInfoEndpointIsRemoved(t *testing.T) {
	database, _, handler := newTestServer(t)
	defer database.Close()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET removed system info status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestReadinessFailsWhenDatabaseIsClosed(t *testing.T) {
	database, _, handler := newTestServer(t)
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "database_unavailable") {
		t.Fatalf("GET /readyz body = %s", response.Body.String())
	}
}

func TestUnknownRouteMatchesWebAvailability(t *testing.T) {
	database, _, handler := newTestServer(t)
	defer database.Close()

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	_, embeddedWebAvailable := webassets.FileSystem()
	if embeddedWebAvailable {
		if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("GET /missing status/type = %d/%q", response.Code, response.Header().Get("Content-Type"))
		}
	} else {
		if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("GET /missing status/type = %d/%q", response.Code, response.Header().Get("Content-Type"))
		}
	}
}

func newTestServer(t *testing.T, checkers ...connections.Checker) (*store.Store, cpconfig.Config, http.Handler) {
	t.Helper()
	root := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(root, "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	config := cpconfig.Config{
		ListenAddress:     "127.0.0.1:0",
		DataDir:           root,
		DatabasePath:      database.Path(),
		RuntimeDir:        filepath.Join(root, "runtime"),
		MasterKey:         bytes.Repeat([]byte{0x61}, 32),
		WorkerPath:        filepath.Join(root, "redis-shake"),
		MaxConcurrentRuns: 4,
		LogRetentionDays:  7,
	}
	cipher, err := secrets.NewCipher(config.MasterKey)
	if err != nil {
		t.Fatalf("secrets.NewCipher() error = %v", err)
	}
	var checker connections.Checker
	if len(checkers) > 0 {
		checker = checkers[0]
	}
	connectionService := connections.NewService(database, cipher, checker)
	taskService := tasks.NewService(database, connectionService, &taskconfig.Renderer{}, config.RuntimeDir)
	server := NewServer(database, config, connectionService, taskService, nil)
	return database, config, server.Handler()
}
