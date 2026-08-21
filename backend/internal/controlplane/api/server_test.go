package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	cpconfig "RedisShake/internal/controlplane/config"
	"RedisShake/internal/controlplane/store"
)

func TestHealthReadyAndSystemInfo(t *testing.T) {
	database, config, handler := newTestServer(t)
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

	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET system info status = %d", response.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode system info: %v", err)
	}
	if payload["version"] != "test-version" || payload["storage"] != "sqlite" {
		t.Fatalf("system info = %#v", payload)
	}
	if payload["data_dir"] != config.DataDir || payload["runtime_dir"] != config.RuntimeDir {
		t.Fatalf("system info paths = %#v", payload)
	}
	if payload["secrets_configured"] != true {
		t.Fatalf("secrets_configured = %#v", payload["secrets_configured"])
	}
	if bytes.Contains(response.Body.Bytes(), config.MasterKey) {
		t.Fatal("system info response leaked the master key")
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

func TestUnknownRouteReturnsJSON(t *testing.T) {
	database, _, handler := newTestServer(t)
	defer database.Close()

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET /missing status = %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("GET /missing content type = %q", response.Header().Get("Content-Type"))
	}
}

func newTestServer(t *testing.T) (*store.Store, cpconfig.Config, http.Handler) {
	t.Helper()
	root := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(root, "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	config := cpconfig.Config{
		ListenAddress: "127.0.0.1:0",
		DataDir:       root,
		DatabasePath:  database.Path(),
		RuntimeDir:    filepath.Join(root, "runtime"),
		MasterKey:     bytes.Repeat([]byte{0x61}, 32),
	}
	server := NewServer(database, config, BuildInfo{Version: "test-version", GitCommit: "abc123"})
	return database, config, server.Handler()
}
