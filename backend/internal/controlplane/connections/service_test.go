package connections

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
)

type fakeChecker struct {
	result TestResult
	seen   Resolved
}

func (f *fakeChecker) Check(_ context.Context, connection Resolved, purpose TestPurpose) TestResult {
	f.seen = connection
	result := f.result
	result.Purpose = purpose
	return result
}

func TestServiceCreateUpdateAndResolveProtectsSecrets(t *testing.T) {
	ctx := context.Background()
	service, database := newTestService(t, &fakeChecker{})
	defer database.Close()

	created, err := service.Create(ctx, Spec{
		Name:     " Production Redis ",
		Topology: domain.TopologyStandalone,
		Address:  "127.0.0.1:6379",
		Username: " app-user ",
		Password: "redis-password",
		TLS: TLSConfig{
			Enabled:            true,
			ServerName:         " redis.internal ",
			InsecureSkipVerify: false,
			CACertPEM:          "ca-certificate",
			ClientCertPEM:      "client-certificate",
			ClientKeyPEM:       "client-private-key",
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Name != "Production Redis" || created.Username != "app-user" {
		t.Fatalf("Create() view = %+v", created)
	}
	if !created.PasswordConfigured || !created.TLS.CACertConfigured || !created.TLS.ClientKeyConfigured {
		t.Fatalf("Create() secret flags = %+v", created)
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("json.Marshal(view) error = %v", err)
	}
	for _, secret := range []string{"redis-password", "ca-certificate", "client-private-key"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("connection view leaked %q: %s", secret, encoded)
		}
	}

	stored, err := database.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	if strings.Contains(stored.PasswordCiphertext, "redis-password") || strings.Contains(stored.TLSConfigJSON, "client-private-key") {
		t.Fatal("stored connection contains plaintext secret material")
	}
	resolved, err := service.Resolve(ctx, created.ID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Password != "redis-password" || resolved.TLS.ClientKeyPEM != "client-private-key" {
		t.Fatalf("Resolve() did not restore secrets: %+v", resolved)
	}

	newName := "Renamed Redis"
	updated, err := service.Update(ctx, created.ID, Patch{Name: &newName})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != newName || !updated.PasswordConfigured || !updated.TLS.ClientKeyConfigured {
		t.Fatalf("Update() did not preserve secrets: %+v", updated)
	}
	emptyPassword := ""
	updated, err = service.Update(ctx, created.ID, Patch{Password: &emptyPassword})
	if err != nil {
		t.Fatalf("Update(clear password) error = %v", err)
	}
	if updated.PasswordConfigured {
		t.Fatal("Update(clear password) left password configured")
	}
	copied, err := service.Copy(ctx, created.ID, "Production Redis Copy")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if copied.ID == created.ID || copied.Name != "Production Redis Copy" || !copied.TLS.ClientKeyConfigured {
		t.Fatalf("Copy() result = %+v", copied)
	}
	resolvedCopy, err := service.Resolve(ctx, copied.ID)
	if err != nil || resolvedCopy.TLS.ClientKeyPEM != "client-private-key" {
		t.Fatalf("Resolve(copy) = %+v/%v", resolvedCopy, err)
	}
}

func TestServiceRejectsSecretWithoutMasterKey(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer database.Close()
	service := NewService(database, nil, nil)

	_, err = service.Create(ctx, Spec{
		Name:     "Protected",
		Topology: domain.TopologyStandalone,
		Address:  "127.0.0.1:6379",
		Password: "secret",
	})
	if !errors.Is(err, ErrSecretsNotConfigured) {
		t.Fatalf("Create() error = %v, want ErrSecretsNotConfigured", err)
	}
}

func TestServiceValidatesTopologyAndTLS(t *testing.T) {
	ctx := context.Background()
	service, database := newTestService(t, nil)
	defer database.Close()

	for _, test := range []struct {
		name  string
		spec  Spec
		field string
	}{
		{
			name:  "invalid address",
			spec:  Spec{Name: "Invalid", Topology: domain.TopologyStandalone, Address: "localhost"},
			field: "address",
		},
		{
			name:  "sentinel master required",
			spec:  Spec{Name: "Sentinel", Topology: domain.TopologySentinel, Sentinel: SentinelConfig{Address: "127.0.0.1:26379"}},
			field: "sentinel.master_name",
		},
		{
			name: "client key pair",
			spec: Spec{
				Name:     "TLS",
				Topology: domain.TopologyStandalone,
				Address:  "127.0.0.1:6379",
				TLS:      TLSConfig{Enabled: true, ClientCertPEM: "cert-only"},
			},
			field: "tls",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Create(ctx, test.spec)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("Create() error = %v, want validation field %q", err, test.field)
			}
		})
	}
}

func TestServiceConnectionTestPersistsSanitizedResult(t *testing.T) {
	ctx := context.Background()
	fixedTime := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	checker := &fakeChecker{result: TestResult{
		Success:  true,
		Checks:   []CheckItem{{Code: "ping", State: CheckStatePass, Message: "ok"}},
		TestedAt: fixedTime,
	}}
	service, database := newTestService(t, checker)
	defer database.Close()
	created, err := service.Create(ctx, Spec{
		Name:     "Test Redis",
		Topology: domain.TopologyStandalone,
		Address:  "127.0.0.1:6379",
		Password: "test-password",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	result, err := service.TestSaved(ctx, created.ID, TestPurposeTarget)
	if err != nil {
		t.Fatalf("TestSaved() error = %v", err)
	}
	if !result.Success || checker.seen.Password != "test-password" {
		t.Fatalf("TestSaved() result = %+v, seen = %+v", result, checker.seen)
	}
	view, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if view.LastTestedAt == nil || !view.LastTestedAt.Equal(fixedTime) || len(view.LastTestResult) == 0 {
		t.Fatalf("Get() test result = %+v", view)
	}
	if strings.Contains(string(view.LastTestResult), "test-password") {
		t.Fatal("stored test result leaked password")
	}
}

func TestServiceDeleteRejectsConnectionUsedByTask(t *testing.T) {
	ctx := context.Background()
	service, database := newTestService(t, nil)
	defer database.Close()
	source, err := service.Create(ctx, Spec{Name: "Source", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6379"})
	if err != nil {
		t.Fatalf("Create(source) error = %v", err)
	}
	target, err := service.Create(ctx, Spec{Name: "Target", Topology: domain.TopologyStandalone, Address: "127.0.0.1:6380"})
	if err != nil {
		t.Fatalf("Create(target) error = %v", err)
	}
	now := time.Now().UTC()
	if err := database.CreateTask(ctx, domain.Task{
		ID:                 "task-1",
		Name:               "Task",
		Mode:               domain.TaskModeScan,
		SourceConnectionID: source.ID,
		TargetConnectionID: target.ID,
		State:              domain.TaskStateDraft,
		ConfigRevision:     1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := service.Delete(ctx, source.ID); !errors.Is(err, store.ErrInUse) {
		t.Fatalf("Delete() error = %v, want ErrInUse", err)
	}
}

func newTestService(t *testing.T, checker Checker) (*Service, *store.Store) {
	t.Helper()
	database, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	cipher, err := secrets.NewCipher(bytes.Repeat([]byte{0x73}, 32))
	if err != nil {
		database.Close()
		t.Fatalf("secrets.NewCipher() error = %v", err)
	}
	return NewService(database, cipher, checker), database
}
