package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/tasks"
)

func TestTaskAPIRevisionPrecheckAndArchive(t *testing.T) {
	checker := &apiFakeChecker{}
	database, _, handler := newTestServer(t, checker)
	defer database.Close()
	source := createAPIConnection(t, handler, "Source", "127.0.0.1:6379")
	target := createAPIConnection(t, handler, "Target", "127.0.0.1:6380")

	create := performJSONRequest(t, handler, http.MethodPost, "/api/v1/tasks", `{"name":"UI Migration","mode":"scan"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("POST task status = %d, body = %s", create.Code, create.Body.String())
	}
	var task tasks.View
	if err := json.Unmarshal(create.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if task.State != domain.TaskStateDraft || task.ConfigRevision != 1 {
		t.Fatalf("created task = %+v", task)
	}
	if task.Spec.ScanReader == nil || task.Spec.ScanReader.DBs == nil || task.Spec.Filter.AllowKeys == nil {
		t.Fatalf("created task returned null collection fields: %+v", task.Spec)
	}

	patchBody := fmt.Sprintf(`{"expected_revision":1,"source_connection_id":%q,"target_connection_id":%q}`, source.ID, target.ID)
	patch := performJSONRequest(t, handler, http.MethodPatch, "/api/v1/tasks/"+task.ID, patchBody)
	if patch.Code != http.StatusOK {
		t.Fatalf("PATCH task status = %d, body = %s", patch.Code, patch.Body.String())
	}
	if err := json.Unmarshal(patch.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode patched task: %v", err)
	}
	if task.ConfigRevision != 2 {
		t.Fatalf("patched revision = %d", task.ConfigRevision)
	}

	stale := performJSONRequest(t, handler, http.MethodPatch, "/api/v1/tasks/"+task.ID, `{"expected_revision":1,"description":"stale"}`)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "revision_conflict") {
		t.Fatalf("stale PATCH status = %d, body = %s", stale.Code, stale.Body.String())
	}

	precheck := performJSONRequest(t, handler, http.MethodPost, "/api/v1/tasks/"+task.ID+"/precheck", `{"expected_revision":2}`)
	if precheck.Code != http.StatusOK || !strings.Contains(precheck.Body.String(), `"ready":true`) || !strings.Contains(precheck.Body.String(), "config_digest") {
		t.Fatalf("POST precheck status = %d, body = %s", precheck.Code, precheck.Body.String())
	}
	get := performJSONRequest(t, handler, http.MethodGet, "/api/v1/tasks/"+task.ID, "")
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"state":"READY"`) {
		t.Fatalf("GET ready task status = %d, body = %s", get.Code, get.Body.String())
	}

	archive := performJSONRequest(t, handler, http.MethodDelete, "/api/v1/tasks/"+task.ID, "")
	if archive.Code != http.StatusNoContent {
		t.Fatalf("DELETE task status = %d, body = %s", archive.Code, archive.Body.String())
	}
	list := performJSONRequest(t, handler, http.MethodGet, "/api/v1/tasks", "")
	if strings.Contains(list.Body.String(), task.ID) {
		t.Fatalf("default task list contains archived task: %s", list.Body.String())
	}
	archivedList := performJSONRequest(t, handler, http.MethodGet, "/api/v1/tasks?include_archived=true", "")
	if !strings.Contains(archivedList.Body.String(), task.ID) {
		t.Fatalf("archived task list missing task: %s", archivedList.Body.String())
	}
}

func createAPIConnection(t *testing.T, handler http.Handler, name, address string) connections.View {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"topology":"standalone","address":%q}`, name, address)
	response := performJSONRequest(t, handler, http.MethodPost, "/api/v1/connections", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("POST connection status = %d, body = %s", response.Code, response.Body.String())
	}
	var connection connections.View
	if err := json.Unmarshal(response.Body.Bytes(), &connection); err != nil {
		t.Fatalf("decode connection: %v", err)
	}
	return connection
}
