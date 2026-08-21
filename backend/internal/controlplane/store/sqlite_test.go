package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/secrets"
)

func TestStoreMigratesAndPersistsModelsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "control-plane.db")
	cipher, err := secrets.NewCipher(bytes.Repeat([]byte{0x51}, 32))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	passwordCiphertext, err := cipher.Encrypt("database-password")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)

	database, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	source := domain.Connection{
		ID:                 "source",
		Name:               "Source Redis",
		Topology:           domain.TopologyStandalone,
		Address:            "127.0.0.1:6379",
		Username:           "sync-user",
		PasswordCiphertext: passwordCiphertext,
		TLSConfigJSON:      `{}`,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	target := source
	target.ID = "target"
	target.Name = "Target Redis"
	target.Address = "127.0.0.1:6380"
	for _, connection := range []domain.Connection{source, target} {
		if err := database.CreateConnection(ctx, connection); err != nil {
			t.Fatalf("CreateConnection(%s) error = %v", connection.ID, err)
		}
	}

	task := domain.Task{
		ID:                  "task-1",
		Name:                "Primary migration",
		Mode:                domain.TaskModeSync,
		SourceConnectionID:  source.ID,
		TargetConnectionID:  target.ID,
		ReaderOptionsJSON:   `{"sync_rdb":true,"sync_aof":true}`,
		FilterOptionsJSON:   `{}`,
		AdvancedOptionsJSON: `{"target_redis_max_qps":1000}`,
		State:               domain.TaskStateReady,
		ConfigRevision:      3,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := database.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	pid := 1234
	statusPort := 19001
	run := domain.Run{
		ID:                  "run-1",
		TaskID:              task.ID,
		ConfigRevision:      task.ConfigRevision,
		ConfigSnapshotJSON:  `{"mode":"sync"}`,
		State:               domain.RunStateRunning,
		PID:                 &pid,
		StatusPort:          &statusPort,
		RuntimeDir:          "/tmp/redisshake/run-1",
		StartedAt:           now,
		LastHeartbeatAt:     &now,
		StopRequestedByUser: false,
	}
	if err := database.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	hasSecrets, err := database.HasEncryptedSecrets(ctx)
	if err != nil {
		t.Fatalf("HasEncryptedSecrets() error = %v", err)
	}
	if !hasSecrets {
		t.Fatal("HasEncryptedSecrets() = false, want true")
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("Open() after restart error = %v", err)
	}
	defer reopened.Close()

	storedConnection, err := reopened.GetConnection(ctx, source.ID)
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	if storedConnection.PasswordCiphertext != passwordCiphertext {
		t.Fatal("GetConnection() returned a different encrypted password")
	}
	if storedConnection.Username != source.Username {
		t.Fatalf("GetConnection().Username = %q", storedConnection.Username)
	}
	storedTask, err := reopened.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if storedTask.ConfigRevision != 3 || storedTask.State != domain.TaskStateReady {
		t.Fatalf("GetTask() = %+v", storedTask)
	}
	storedRun, err := reopened.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if storedRun.PID == nil || *storedRun.PID != pid || storedRun.State != domain.RunStateRunning {
		t.Fatalf("GetRun() = %+v", storedRun)
	}

	contents, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatalf("ReadFile(database) error = %v", err)
	}
	if strings.Contains(string(contents), "database-password") {
		t.Fatal("SQLite file contains the plaintext password")
	}
	info, err := os.Stat(databasePath)
	if err != nil {
		t.Fatalf("Stat(database) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("database permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestStoreEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "control-plane.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	now := time.Now().UTC()
	err = database.CreateTask(ctx, domain.Task{
		ID:                 "orphan-task",
		Name:               "Orphan",
		Mode:               domain.TaskModeScan,
		SourceConnectionID: "missing-source",
		TargetConnectionID: "missing-target",
		State:              domain.TaskStateDraft,
		ConfigRevision:     1,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err == nil {
		t.Fatal("CreateTask() accepted missing connection references")
	}
}

func TestStoreReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "control-plane.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	if _, err := database.GetConnection(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetConnection() error = %v, want ErrNotFound", err)
	}
}
