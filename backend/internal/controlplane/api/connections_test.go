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
	"time"

	cpconfig "RedisShake/internal/controlplane/config"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/store"
	"RedisShake/internal/controlplane/taskconfig"
	"RedisShake/internal/controlplane/tasks"
)

type apiFakeChecker struct {
	seen connections.Resolved
}

func (f *apiFakeChecker) Check(_ context.Context, resolved connections.Resolved, purpose connections.TestPurpose) connections.TestResult {
	f.seen = resolved
	return connections.TestResult{
		Success:          true,
		Purpose:          purpose,
		EffectiveAddress: resolved.Address,
		ServerProduct:    "Redis",
		ServerVersion:    "7.2.0",
		Role:             "master",
		Checks:           []connections.CheckItem{{Code: "ping", State: connections.CheckStatePass, Message: "ok"}},
		TestedAt:         time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC),
	}
}

func TestConnectionCRUDNeverReturnsCredentials(t *testing.T) {
	database, _, handler := newTestServer(t)
	defer database.Close()

	createBody := `{
		"name":"Primary Redis",
		"topology":"standalone",
		"address":"127.0.0.1:6379",
		"username":"app-user",
		"password":"api-password",
		"tls":{"enabled":false},
		"sentinel":{"tls":{"enabled":false}}
	}`
	createResponse := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", createBody)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("POST connection status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	assertNoCredentialLeak(t, createResponse.Body.Bytes())
	var created connections.View
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created connection: %v", err)
	}
	if created.ID == "" || !created.PasswordConfigured {
		t.Fatalf("created connection = %+v", created)
	}

	listResponse := performJSONRequest(t, handler, http.MethodGet, "/api/v1/connections", "")
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), created.ID) {
		t.Fatalf("GET connections status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	assertNoCredentialLeak(t, listResponse.Body.Bytes())

	patchResponse := performJSONRequest(t, handler, http.MethodPatch, "/api/v1/connections/"+created.ID, `{"name":"Renamed Redis"}`)
	if patchResponse.Code != http.StatusOK || !strings.Contains(patchResponse.Body.String(), "Renamed Redis") {
		t.Fatalf("PATCH connection status = %d, body = %s", patchResponse.Code, patchResponse.Body.String())
	}
	assertNoCredentialLeak(t, patchResponse.Body.Bytes())
	copyResponse := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections/"+created.ID+"/copy", `{"name":"Copied Redis"}`)
	if copyResponse.Code != http.StatusCreated || !strings.Contains(copyResponse.Body.String(), "Copied Redis") {
		t.Fatalf("POST connection copy status = %d, body = %s", copyResponse.Code, copyResponse.Body.String())
	}
	assertNoCredentialLeak(t, copyResponse.Body.Bytes())

	deleteResponse := performJSONRequest(t, handler, http.MethodDelete, "/api/v1/connections/"+created.ID, "")
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("DELETE connection status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	getResponse := performJSONRequest(t, handler, http.MethodGet, "/api/v1/connections/"+created.ID, "")
	if getResponse.Code != http.StatusNotFound {
		t.Fatalf("GET deleted connection status = %d", getResponse.Code)
	}
}

func TestConnectionConflictAndValidationErrors(t *testing.T) {
	database, _, handler := newTestServer(t)
	defer database.Close()
	body := `{"name":"Duplicate","topology":"standalone","address":"127.0.0.1:6379"}`
	if response := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", body); response.Code != http.StatusCreated {
		t.Fatalf("first create status = %d", response.Code)
	}
	if response := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", body); response.Code != http.StatusConflict {
		t.Fatalf("duplicate create status = %d, body = %s", response.Code, response.Body.String())
	}
	invalid := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", `{"name":"Invalid","topology":"standalone","address":"missing-port"}`)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"field":"address"`) {
		t.Fatalf("invalid create status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
	unknown := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", `{"name":"Invalid","unknown":true}`)
	if unknown.Code != http.StatusBadRequest || !strings.Contains(unknown.Body.String(), "invalid_json") {
		t.Fatalf("unknown field status = %d, body = %s", unknown.Code, unknown.Body.String())
	}
}

func TestConnectionCredentialRequiresMasterKey(t *testing.T) {
	root := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(root, "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer database.Close()
	config := cpconfig.Config{DataDir: root, RuntimeDir: filepath.Join(root, "runtime")}
	service := connections.NewService(database, nil, nil)
	taskService := tasks.NewService(database, service, &taskconfig.Renderer{}, config.RuntimeDir)
	handler := NewServer(database, config, service, taskService, nil).Handler()

	response := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", `{
		"name":"Protected",
		"topology":"standalone",
		"address":"127.0.0.1:6379",
		"password":"must-not-leak"
	}`)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "master_key_required") {
		t.Fatalf("POST protected connection status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "must-not-leak") {
		t.Fatal("master key error leaked the submitted password")
	}
}

func TestSavedAndUnsavedConnectionTests(t *testing.T) {
	checker := &apiFakeChecker{}
	database, _, handler := newTestServer(t, checker)
	defer database.Close()
	create := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", `{
		"name":"Checked Redis",
		"topology":"standalone",
		"address":"127.0.0.1:6379",
		"password":"checker-password"
	}`)
	var created connections.View
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created connection: %v", err)
	}

	saved := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections/"+created.ID+"/test", `{"purpose":"target"}`)
	if saved.Code != http.StatusOK || !strings.Contains(saved.Body.String(), `"success":true`) {
		t.Fatalf("saved test status = %d, body = %s", saved.Code, saved.Body.String())
	}
	if checker.seen.Password != "checker-password" {
		t.Fatal("saved test checker did not receive decrypted password")
	}
	assertNoCredentialLeak(t, saved.Body.Bytes())

	unsaved := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections/test", `{
		"purpose":"source",
		"connection":{"name":"Unsaved","topology":"standalone","address":"127.0.0.1:6380","password":"unsaved-password"}
	}`)
	if unsaved.Code != http.StatusOK || checker.seen.Password != "unsaved-password" {
		t.Fatalf("unsaved test status = %d, body = %s", unsaved.Code, unsaved.Body.String())
	}
	assertNoCredentialLeak(t, unsaved.Body.Bytes())
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertNoCredentialLeak(t *testing.T, body []byte) {
	t.Helper()
	for _, secret := range [][]byte{[]byte("api-password"), []byte("checker-password"), []byte("unsaved-password"), []byte("ciphertext")} {
		if bytes.Contains(body, secret) {
			t.Fatalf("response leaked credential material: %s", body)
		}
	}
}
