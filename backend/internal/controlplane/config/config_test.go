package config

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesConfiguredDirectories(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "control-plane")
	runtimeDir := filepath.Join(root, "worker-runtime")
	databasePath := filepath.Join(root, "database", "state.db")
	key := bytes.Repeat([]byte{0x2a}, 32)

	t.Setenv(EnvDataDir, dataDir)
	t.Setenv(EnvRuntimeDir, runtimeDir)
	t.Setenv(EnvDatabasePath, databasePath)
	t.Setenv(EnvListenAddress, "127.0.0.1:18080")
	t.Setenv(EnvMasterKey, base64.StdEncoding.EncodeToString(key))

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.ListenAddress != "127.0.0.1:18080" {
		t.Fatalf("ListenAddress = %q", config.ListenAddress)
	}
	if config.DatabasePath != databasePath {
		t.Fatalf("DatabasePath = %q, want %q", config.DatabasePath, databasePath)
	}
	if !config.SecretsConfigured() {
		t.Fatal("SecretsConfigured() = false, want true")
	}
	for _, dir := range []string{dataDir, runtimeDir, filepath.Dir(databasePath)} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %q: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", dir)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("permissions for %q = %o, want 700", dir, info.Mode().Perm())
		}
	}
}

func TestLoadRejectsInvalidMasterKey(t *testing.T) {
	t.Setenv(EnvDataDir, t.TempDir())
	t.Setenv(EnvMasterKey, base64.StdEncoding.EncodeToString([]byte("too-short")))

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid master key error")
	}
}

func TestLoadDefaultsToLoopback(t *testing.T) {
	t.Setenv(EnvDataDir, t.TempDir())
	t.Setenv(EnvRuntimeDir, "")
	t.Setenv(EnvDatabasePath, "")
	t.Setenv(EnvListenAddress, "")
	t.Setenv(EnvMasterKey, "")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.ListenAddress != "127.0.0.1:8080" {
		t.Fatalf("ListenAddress = %q", config.ListenAddress)
	}
	if config.SecretsConfigured() {
		t.Fatal("SecretsConfigured() = true without a key")
	}
}
