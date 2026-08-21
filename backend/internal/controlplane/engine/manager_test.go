package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
	"RedisShake/internal/controlplane/taskconfig"
	"RedisShake/internal/controlplane/tasks"
)

type engineFakeChecker struct{}

func (f *engineFakeChecker) Check(_ context.Context, resolved connections.Resolved, purpose connections.TestPurpose) connections.TestResult {
	return connections.TestResult{
		Success:          true,
		Purpose:          purpose,
		EffectiveAddress: resolved.Address,
		ServerProduct:    "Redis",
		ServerVersion:    "7.2.0",
		Role:             "master",
		Checks:           []connections.CheckItem{{Code: "ping", State: connections.CheckStatePass, Message: "ok"}},
		TestedAt:         time.Now().UTC(),
	}
}

func TestManagerScanCompletesAndPersistsStatus(t *testing.T) {
	manager, task, database := newEngineTestManager(t, domain.TaskModeScan)
	defer database.Close()
	ctx := context.Background()
	run, err := manager.Start(ctx, task.ID, StartRequest{ExpectedRevision: task.ConfigRevision})
	if err != nil {
		t.Fatalf("Start() error = %v, run = %+v", err, run)
	}
	completed := waitForRunState(t, manager, run.ID, domain.RunStateSucceeded)
	if completed.ExitCode == nil || *completed.ExitCode != 0 || completed.ExitReason != "scan completed" {
		t.Fatalf("completed run = %+v", completed)
	}
	if len(completed.Status) == 0 || completed.LastHeartbeatAt == nil {
		t.Fatalf("completed run missing captured status = %+v", completed)
	}
	if _, err := manager.Stop(ctx, run.ID); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("Stop(terminal run) error = %v", err)
	}
	logs, err := manager.ReadLogs(ctx, run.ID, 0, 65536)
	if err != nil {
		t.Fatalf("ReadLogs() error = %v", err)
	}
	if !strings.Contains(logs.Content, "fake worker started") || strings.Contains(logs.Content, "worker-secret") {
		t.Fatalf("logs were not captured and redacted: %q", logs.Content)
	}
	stored, err := database.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if strings.Contains(stored.ConfigSnapshotJSON, "source-password") || strings.Contains(stored.ConfigSnapshotJSON, "target-password") {
		t.Fatal("run config snapshot contains connection credentials")
	}
	for _, name := range []string{configFileName, stdoutFileName, processFileName} {
		if _, err := os.Stat(filepath.Join(stored.RuntimeDir, name)); err != nil {
			t.Fatalf("run artifact %s missing: %v", name, err)
		}
	}
}

func TestManagerSyncStopAndDuplicateProtection(t *testing.T) {
	manager, task, database := newEngineTestManager(t, domain.TaskModeSync)
	defer database.Close()
	ctx := context.Background()
	run, err := manager.Start(ctx, task.ID, StartRequest{ExpectedRevision: task.ConfigRevision})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if run.State != domain.RunStateRunning {
		t.Fatalf("started run state = %s", run.State)
	}
	if _, err := manager.Start(ctx, task.ID, StartRequest{ExpectedRevision: task.ConfigRevision}); !errors.Is(err, store.ErrActiveRun) {
		t.Fatalf("duplicate Start() error = %v", err)
	}
	stopping, err := manager.Stop(ctx, run.ID)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if stopping.State != domain.RunStateStopping {
		t.Fatalf("stopping state = %s", stopping.State)
	}
	stopped := waitForRunState(t, manager, run.ID, domain.RunStateStopped)
	if stopped.ExitReason != "stopped by user" || !stopped.StopRequestedByUser {
		t.Fatalf("stopped run = %+v", stopped)
	}
	second, err := manager.Start(ctx, task.ID, StartRequest{ExpectedRevision: task.ConfigRevision})
	if err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	if _, err := manager.ForceStop(ctx, second.ID); err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}
	forceStopped := waitForRunState(t, manager, second.ID, domain.RunStateStopped)
	if forceStopped.ExitReason != "force-stopped by user" {
		t.Fatalf("force-stopped run = %+v", forceStopped)
	}
}

func TestManagerRestartMarksRunUnknownAndDoesNotSignalIt(t *testing.T) {
	manager, task, database := newEngineTestManager(t, domain.TaskModeSync)
	defer database.Close()
	ctx := context.Background()
	run, err := manager.Start(ctx, task.ID, StartRequest{ExpectedRevision: task.ConfigRevision})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	restarted := NewManager(database, manager.tasks, manager.connections, manager.renderer, manager.config)
	count, err := restarted.Initialize(ctx)
	if err != nil || count != 1 {
		t.Fatalf("Initialize() count/error = %d/%v", count, err)
	}
	unknown, err := restarted.Get(ctx, run.ID)
	if err != nil || unknown.State != domain.RunStateUnknown {
		t.Fatalf("unknown run = %+v, err = %v", unknown, err)
	}
	if _, err := restarted.Stop(ctx, run.ID); !errors.Is(err, ErrRunNotManaged) {
		t.Fatalf("restarted Stop() error = %v", err)
	}
	if _, err := restarted.Start(ctx, task.ID, StartRequest{ExpectedRevision: task.ConfigRevision}); !errors.Is(err, store.ErrActiveRun) {
		t.Fatalf("Start() with unknown run error = %v", err)
	}
	shutdownContext, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := manager.Shutdown(shutdownContext); err != nil {
		t.Fatalf("original manager Shutdown() error = %v", err)
	}
}

func TestManagerWorkerUnavailable(t *testing.T) {
	manager, task, database := newEngineTestManager(t, domain.TaskModeScan)
	defer database.Close()
	manager.config.WorkerPath = filepath.Join(t.TempDir(), "missing-worker")
	if _, err := manager.Start(context.Background(), task.ID, StartRequest{ExpectedRevision: task.ConfigRevision}); !errors.Is(err, ErrWorkerUnavailable) {
		t.Fatalf("Start() error = %v, want ErrWorkerUnavailable", err)
	}
}

func newEngineTestManager(t *testing.T, mode domain.TaskMode) (*Manager, tasks.View, *store.Store) {
	t.Helper()
	root := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(root, "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	cipher, err := secrets.NewCipher(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		database.Close()
		t.Fatalf("secrets.NewCipher() error = %v", err)
	}
	connectionService := connections.NewService(database, cipher, &engineFakeChecker{})
	source, err := connectionService.Create(context.Background(), connections.Spec{
		Name: "Source", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6379", Password: "source-password",
	})
	if err != nil {
		database.Close()
		t.Fatalf("Create(source) error = %v", err)
	}
	target, err := connectionService.Create(context.Background(), connections.Spec{
		Name: "Target", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6380", Password: "target-password",
	})
	if err != nil {
		database.Close()
		t.Fatalf("Create(target) error = %v", err)
	}
	renderer := &taskconfig.Renderer{}
	taskService := tasks.NewService(database, connectionService, renderer, filepath.Join(root, "runtime"))
	spec := tasks.Spec{
		Name:               "Task",
		Mode:               mode,
		SourceConnectionID: source.ID,
		TargetConnectionID: target.ID,
	}
	task, err := taskService.Create(context.Background(), spec)
	if err != nil {
		database.Close()
		t.Fatalf("Create(task) error = %v", err)
	}
	precheck, err := taskService.Precheck(context.Background(), task.ID, tasks.PrecheckRequest{ExpectedRevision: task.ConfigRevision})
	if err != nil || !precheck.Ready {
		database.Close()
		t.Fatalf("Precheck() result/error = %+v/%v", precheck, err)
	}
	task, err = taskService.Get(context.Background(), task.ID)
	if err != nil {
		database.Close()
		t.Fatalf("Get(task) error = %v", err)
	}
	workerPath := buildFakeWorker(t)
	manager := NewManager(database, taskService, connectionService, renderer, ManagerConfig{
		WorkerPath:   workerPath,
		RuntimeDir:   filepath.Join(root, "runtime"),
		StartTimeout: 5 * time.Second,
		StopTimeout:  2 * time.Second,
	})
	return manager, task, database
}

func buildFakeWorker(t *testing.T) string {
	t.Helper()
	extension := ""
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}
	path := filepath.Join(t.TempDir(), "fake-worker"+extension)
	command := exec.Command("go", "build", "-o", path, "./testdata/fakeworker")
	command.Dir = "."
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake worker: %v\n%s", err, output)
	}
	return path
}

func waitForRunState(t *testing.T, manager *Manager, runID string, expected domain.RunState) RunView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		run, err := manager.Get(context.Background(), runID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if run.State == expected {
			return run
		}
		time.Sleep(50 * time.Millisecond)
	}
	run, _ := manager.Get(context.Background(), runID)
	t.Fatalf("run did not reach %s: %+v", expected, run)
	return RunView{}
}
