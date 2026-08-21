package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"RedisShake/internal/controlplane/secrets"
)

const (
	EnvListenAddress = "REDISSHAKE_LISTEN_ADDR"
	EnvDataDir       = "REDISSHAKE_DATA_DIR"
	EnvDatabasePath  = "REDISSHAKE_DB_PATH"
	EnvRuntimeDir    = "REDISSHAKE_RUNTIME_DIR"
	EnvMasterKey     = "REDISSHAKE_MASTER_KEY"
)

type Config struct {
	ListenAddress string
	DataDir       string
	DatabasePath  string
	RuntimeDir    string
	MasterKey     []byte
}

func Load() (Config, error) {
	dataDir := strings.TrimSpace(os.Getenv(EnvDataDir))
	if dataDir == "" {
		dataDir = "data"
	}
	absoluteDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve data directory: %w", err)
	}

	databasePath := strings.TrimSpace(os.Getenv(EnvDatabasePath))
	if databasePath == "" {
		databasePath = filepath.Join(absoluteDataDir, "control-plane.db")
	} else if databasePath, err = filepath.Abs(databasePath); err != nil {
		return Config{}, fmt.Errorf("resolve database path: %w", err)
	}

	runtimeDir := strings.TrimSpace(os.Getenv(EnvRuntimeDir))
	if runtimeDir == "" {
		runtimeDir = filepath.Join(absoluteDataDir, "runtime")
	} else if runtimeDir, err = filepath.Abs(runtimeDir); err != nil {
		return Config{}, fmt.Errorf("resolve runtime directory: %w", err)
	}

	for _, dir := range []string{absoluteDataDir, filepath.Dir(databasePath), runtimeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Config{}, fmt.Errorf("create data directory %q: %w", dir, err)
		}
	}

	masterKey, err := secrets.DecodeMasterKey(os.Getenv(EnvMasterKey))
	if err != nil {
		return Config{}, err
	}

	listenAddress := strings.TrimSpace(os.Getenv(EnvListenAddress))
	if listenAddress == "" {
		listenAddress = "127.0.0.1:8080"
	}

	return Config{
		ListenAddress: listenAddress,
		DataDir:       absoluteDataDir,
		DatabasePath:  databasePath,
		RuntimeDir:    runtimeDir,
		MasterKey:     masterKey,
	}, nil
}

func (c Config) SecretsConfigured() bool {
	return len(c.MasterKey) == 32
}
