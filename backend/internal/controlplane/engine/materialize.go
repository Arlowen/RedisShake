package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"RedisShake/internal/controlplane/tasks"
)

const (
	configFileName  = "shake.toml"
	stdoutFileName  = "stdout.log"
	processFileName = "process.json"
)

func materializeArtifact(runDir string, artifact tasks.Artifact) (string, string, error) {
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create run directory: %w", err)
	}
	if err := os.Chmod(runDir, 0o700); err != nil {
		return "", "", fmt.Errorf("protect run directory: %w", err)
	}
	dataDir := filepath.Join(runDir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create worker data directory: %w", err)
	}
	for path, contents := range artifact.SecretFiles {
		if err := ensureInside(runDir, path); err != nil {
			return "", "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", "", fmt.Errorf("create secret directory: %w", err)
		}
		if err := writeProtectedFile(path, contents); err != nil {
			return "", "", err
		}
	}
	configPath := filepath.Join(runDir, configFileName)
	if err := writeProtectedFile(configPath, artifact.TOML); err != nil {
		return "", "", err
	}
	stdoutPath := filepath.Join(runDir, stdoutFileName)
	return configPath, stdoutPath, nil
}

func writeProcessMetadata(runDir string, metadata any) error {
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode process metadata: %w", err)
	}
	return writeProtectedFile(filepath.Join(runDir, processFileName), encoded)
}

func writeProtectedFile(path string, contents []byte) error {
	temporaryPath := path + ".tmp"
	file, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create protected file %q: %w", path, err)
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write protected file %q: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync protected file %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close protected file %q: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish protected file %q: %w", path, err)
	}
	cleanup = false
	return nil
}

func ensureInside(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve artifact path %q: %w", path, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("artifact path %q escapes run directory", path)
	}
	return nil
}
