package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"RedisShake/internal/controlplane/tasks"
)

func TestMaterializeArtifactProtectsFilesAndRejectsEscapes(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	secretPath := filepath.Join(runDir, "certs", "source-ca.pem")
	configPath, stdoutPath, err := materializeArtifact(runDir, tasks.Artifact{
		TOML:        []byte("[advanced]\ndir='data'\n"),
		SecretFiles: map[string][]byte{secretPath: []byte("certificate")},
	})
	if err != nil {
		t.Fatalf("materializeArtifact() error = %v", err)
	}
	if configPath != filepath.Join(runDir, configFileName) || stdoutPath != filepath.Join(runDir, stdoutFileName) {
		t.Fatalf("paths = %q/%q", configPath, stdoutPath)
	}
	for _, path := range []string{runDir, filepath.Join(runDir, "data"), filepath.Dir(secretPath)} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat directory %q: %v", path, err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("directory %q permissions = %o", path, info.Mode().Perm())
		}
	}
	for _, path := range []string{configPath, secretPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat file %q: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file %q permissions = %o", path, info.Mode().Perm())
		}
	}

	escape := filepath.Join(runDir, "..", "escaped.pem")
	_, _, err = materializeArtifact(filepath.Join(t.TempDir(), "second-run"), tasks.Artifact{
		TOML:        []byte("config"),
		SecretFiles: map[string][]byte{escape: []byte("secret")},
	})
	if err == nil || !strings.Contains(err.Error(), "escapes run directory") {
		t.Fatalf("materializeArtifact(escape) error = %v", err)
	}
}

func TestRedactingWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdout.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	writer := &redactingWriter{file: file}
	if _, err := writer.Write([]byte("redis://user:pass@host password=secret token=value\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, value := range []string{"pass@", "password=secret", "token=value"} {
		if strings.Contains(string(contents), value) {
			t.Fatalf("redacted log leaked %q: %s", value, contents)
		}
	}
}
