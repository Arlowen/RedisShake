package tasks

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
)

type taskFakeChecker struct {
	calls int
}

func (f *taskFakeChecker) Check(_ context.Context, resolved connections.Resolved, purpose connections.TestPurpose) connections.TestResult {
	f.calls++
	return connections.TestResult{
		Success:          true,
		Purpose:          purpose,
		EffectiveAddress: resolved.Address,
		ServerProduct:    "Redis",
		ServerVersion:    "7.2.0",
		Role:             "master",
		ClusterEnabled:   resolved.Topology == domain.TopologyCluster,
		Checks: []connections.CheckItem{
			{Code: "ping", State: connections.CheckStatePass, Message: "PING 正常"},
		},
		TestedAt: time.Now().UTC(),
	}
}

type taskFakeRenderer struct {
	err            error
	seenSourcePass string
}

func (f *taskFakeRenderer) Render(_ Spec, source, _ connections.Resolved, _ RuntimeConfig) (Artifact, error) {
	f.seenSourcePass = source.Password
	if f.err != nil {
		return Artifact{}, f.err
	}
	return Artifact{TOML: []byte("generated redis-shake config")}, nil
}

func TestTaskDraftRevisionAndPrecheckLifecycle(t *testing.T) {
	ctx := context.Background()
	service, connectionService, database, checker, renderer := newTaskTestService(t)
	defer database.Close()

	draft, err := service.Create(ctx, Spec{Name: " Migration ", Mode: domain.TaskModeSync})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if draft.State != domain.TaskStateDraft || draft.ConfigRevision != 1 || draft.Spec.SyncReader == nil {
		t.Fatalf("Create() draft = %+v", draft)
	}
	source, err := connectionService.Create(ctx, connections.Spec{
		Name:     "Source",
		Topology: domain.TopologyStandalone,
		Address:  "127.0.0.1:6379",
		Password: "source-password",
	})
	if err != nil {
		t.Fatalf("Create(source) error = %v", err)
	}
	target, err := connectionService.Create(ctx, connections.Spec{
		Name:     "Target",
		Topology: domain.TopologyStandalone,
		Address:  "127.0.0.1:6380",
	})
	if err != nil {
		t.Fatalf("Create(target) error = %v", err)
	}
	updated, err := service.Update(ctx, draft.ID, Patch{
		ExpectedRevision:   1,
		SourceConnectionID: &source.ID,
		TargetConnectionID: &target.ID,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.ConfigRevision != 2 || updated.State != domain.TaskStateDraft {
		t.Fatalf("Update() task = %+v", updated)
	}
	if _, err := service.Update(ctx, draft.ID, Patch{ExpectedRevision: 1}); !errors.Is(err, store.ErrRevisionConflict) {
		t.Fatalf("stale Update() error = %v", err)
	}

	precheck, err := service.Precheck(ctx, draft.ID, PrecheckRequest{ExpectedRevision: 2})
	if err != nil {
		t.Fatalf("Precheck() error = %v", err)
	}
	if !precheck.Ready || precheck.ConfigDigest == "" || checker.calls != 2 {
		t.Fatalf("Precheck() result = %+v, calls = %d", precheck, checker.calls)
	}
	if renderer.seenSourcePass != "source-password" {
		t.Fatal("renderer did not receive decrypted source password")
	}
	ready, err := service.Get(ctx, draft.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ready.State != domain.TaskStateReady || ready.LastPrecheckedAt == nil || len(ready.LastPrecheckResult) == 0 {
		t.Fatalf("ready task = %+v", ready)
	}
	if strings.Contains(string(ready.LastPrecheckResult), "source-password") {
		t.Fatal("stored precheck result leaked a connection password")
	}
	copied, err := service.Copy(ctx, draft.ID, "Migration Copy")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if copied.ID == draft.ID || copied.State != domain.TaskStateDraft || copied.ConfigRevision != 1 || copied.LastPrecheckedAt != nil {
		t.Fatalf("Copy() task = %+v", copied)
	}

	newDescription := "changed"
	changed, err := service.Update(ctx, draft.ID, Patch{ExpectedRevision: 2, Description: &newDescription})
	if err != nil {
		t.Fatalf("Update(ready task) error = %v", err)
	}
	if changed.State != domain.TaskStateDraft || changed.ConfigRevision != 3 || changed.LastPrecheckedAt != nil || len(changed.LastPrecheckResult) != 0 {
		t.Fatalf("changed task did not invalidate precheck = %+v", changed)
	}
}

func TestTaskPrecheckRequiresWarningAcknowledgement(t *testing.T) {
	ctx := context.Background()
	service, connectionService, database, _, _ := newTaskTestService(t)
	defer database.Close()
	source, _ := connectionService.Create(ctx, connections.Spec{Name: "Source", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6379"})
	target, _ := connectionService.Create(ctx, connections.Spec{Name: "Target", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6380"})
	task, err := service.Create(ctx, Spec{
		Name:               "Dangerous",
		Mode:               domain.TaskModeScan,
		SourceConnectionID: source.ID,
		TargetConnectionID: target.ID,
		Advanced:           AdvancedOptions{EmptyDBBeforeSync: true},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	first, err := service.Precheck(ctx, task.ID, PrecheckRequest{ExpectedRevision: 1})
	if err != nil {
		t.Fatalf("Precheck() error = %v", err)
	}
	if first.Ready || !hasState(first.Checks, connections.CheckStateWarning) {
		t.Fatalf("unacknowledged precheck = %+v", first)
	}
	second, err := service.Precheck(ctx, task.ID, PrecheckRequest{ExpectedRevision: 1, AcknowledgeWarnings: true})
	if err != nil {
		t.Fatalf("Precheck(acknowledged) error = %v", err)
	}
	if !second.Ready {
		t.Fatalf("acknowledged precheck = %+v", second)
	}
}

func TestTaskPrecheckStopsBeforeConnectionsOnInvalidSpec(t *testing.T) {
	ctx := context.Background()
	service, _, database, checker, _ := newTaskTestService(t)
	defer database.Close()
	task, err := service.Create(ctx, Spec{
		Name: "Incomplete",
		Mode: domain.TaskModeSync,
		Filter: FilterOptions{
			AllowDB: []int{0},
			BlockDB: []int{1},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	result, err := service.Precheck(ctx, task.ID, PrecheckRequest{ExpectedRevision: 1})
	if err != nil {
		t.Fatalf("Precheck() error = %v", err)
	}
	if result.Ready || checker.calls != 0 || !hasFailure(result.Checks) {
		t.Fatalf("invalid precheck = %+v, checker calls = %d", result, checker.calls)
	}
}

func TestTaskPrecheckRendererFailureKeepsDraft(t *testing.T) {
	ctx := context.Background()
	service, connectionService, database, _, renderer := newTaskTestService(t)
	defer database.Close()
	renderer.err = errors.New("render failed")
	source, _ := connectionService.Create(ctx, connections.Spec{Name: "Source", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6379"})
	target, _ := connectionService.Create(ctx, connections.Spec{Name: "Target", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6380"})
	task, _ := service.Create(ctx, Spec{Name: "Render fail", Mode: domain.TaskModeScan, SourceConnectionID: source.ID, TargetConnectionID: target.ID})
	result, err := service.Precheck(ctx, task.ID, PrecheckRequest{ExpectedRevision: 1})
	if err != nil {
		t.Fatalf("Precheck() error = %v", err)
	}
	if result.Ready || !hasFailure(result.Checks) {
		t.Fatalf("renderer failure precheck = %+v", result)
	}
}

func TestTaskArchiveRejectsActiveRun(t *testing.T) {
	ctx := context.Background()
	service, connectionService, database, _, _ := newTaskTestService(t)
	defer database.Close()
	source, _ := connectionService.Create(ctx, connections.Spec{Name: "Source", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6379"})
	target, _ := connectionService.Create(ctx, connections.Spec{Name: "Target", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6380"})
	task, err := service.Create(ctx, Spec{Name: "Active", Mode: domain.TaskModeSync, SourceConnectionID: source.ID, TargetConnectionID: target.ID})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	now := time.Now().UTC()
	if err := database.CreateRun(ctx, domain.Run{
		ID:             "active-run",
		TaskID:         task.ID,
		ConfigRevision: task.ConfigRevision,
		State:          domain.RunStateRunning,
		RuntimeDir:     "/tmp/active-run",
		StartedAt:      now,
	}); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := service.Archive(ctx, task.ID); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("Archive() error = %v, want ErrInUse", err)
	}
}

func newTaskTestService(t *testing.T) (*Service, *connections.Service, *store.Store, *taskFakeChecker, *taskFakeRenderer) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	cipher, err := secrets.NewCipher(bytes.Repeat([]byte{0x29}, 32))
	if err != nil {
		database.Close()
		t.Fatalf("secrets.NewCipher() error = %v", err)
	}
	checker := &taskFakeChecker{}
	connectionService := connections.NewService(database, cipher, checker)
	renderer := &taskFakeRenderer{}
	service := NewService(database, connectionService, renderer, filepath.Join(t.TempDir(), "runtime"))
	return service, connectionService, database, checker, renderer
}
