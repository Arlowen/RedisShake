package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cpconfig "RedisShake/internal/controlplane/config"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
	"RedisShake/internal/controlplane/taskconfig"
	"RedisShake/internal/controlplane/tasks"
)

func TestSPAHandlerServesAssetsAndPreservesAPINotFound(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	if err := os.MkdirAll(filepath.Join(webDir, "assets"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<main>RedisShake UI</main>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "assets", "app.js"), []byte("console.log('ui')"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	database, err := store.Open(context.Background(), filepath.Join(root, "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer database.Close()
	cipher, _ := secrets.NewCipher(bytes.Repeat([]byte{0x22}, 32))
	connectionService := connections.NewService(database, cipher, nil)
	config := cpconfig.Config{DataDir: root, RuntimeDir: filepath.Join(root, "runtime"), WebDir: webDir}
	taskService := tasks.NewService(database, connectionService, &taskconfig.Renderer{}, config.RuntimeDir)
	handler := NewServer(database, config, connectionService, taskService, nil).Handler()

	for _, path := range []string{"/", "/tasks/task-1"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "RedisShake UI") {
			t.Fatalf("GET %s status/body = %d/%s", path, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Header().Get("Content-Security-Policy"), "script-src 'self'") {
			t.Fatalf("GET %s CSP = %q", path, response.Header().Get("Content-Security-Policy"))
		}
	}
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset status/cache = %d/%q", asset.Code, asset.Header().Get("Cache-Control"))
	}
	unknownAPI := httptest.NewRecorder()
	handler.ServeHTTP(unknownAPI, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))
	if unknownAPI.Code != http.StatusNotFound || !strings.Contains(unknownAPI.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unknown API status/type = %d/%q", unknownAPI.Code, unknownAPI.Header().Get("Content-Type"))
	}
	missingAsset := httptest.NewRecorder()
	handler.ServeHTTP(missingAsset, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if missingAsset.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d", missingAsset.Code)
	}
	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/tasks", io.NopCloser(strings.NewReader(""))))
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST SPA route status = %d", method.Code)
	}
}
