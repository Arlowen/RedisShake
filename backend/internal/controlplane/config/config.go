package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"RedisShake/internal/controlplane/secrets"
)

const (
	EnvListenAddress = "REDISSHAKE_LISTEN_ADDR"
	EnvDataDir       = "REDISSHAKE_DATA_DIR"
	EnvDatabasePath  = "REDISSHAKE_DB_PATH"
	EnvRuntimeDir    = "REDISSHAKE_RUNTIME_DIR"
	EnvMasterKey     = "REDISSHAKE_MASTER_KEY"
	EnvWorkerPath    = "REDISSHAKE_WORKER_PATH"
	EnvStartTimeout  = "REDISSHAKE_RUN_START_TIMEOUT"
	EnvStopTimeout   = "REDISSHAKE_RUN_STOP_TIMEOUT"
)

type Config struct {
	ListenAddress string
	DataDir       string
	DatabasePath  string
	RuntimeDir    string
	MasterKey     []byte
	WorkerPath    string
	StartTimeout  time.Duration
	StopTimeout   time.Duration
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
	workerPath := strings.TrimSpace(os.Getenv(EnvWorkerPath))
	if workerPath == "" {
		workerPath = filepath.Join("bin", "redis-shake")
	}
	workerPath, err = filepath.Abs(workerPath)
	if err != nil {
		return Config{}, fmt.Errorf("resolve RedisShake worker path: %w", err)
	}
	startTimeout, err := durationFromEnvironment(EnvStartTimeout, 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	stopTimeout, err := durationFromEnvironment(EnvStopTimeout, 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ListenAddress: listenAddress,
		DataDir:       absoluteDataDir,
		DatabasePath:  databasePath,
		RuntimeDir:    runtimeDir,
		MasterKey:     masterKey,
		WorkerPath:    workerPath,
		StartTimeout:  startTimeout,
		StopTimeout:   stopTimeout,
	}, nil
}

func durationFromEnvironment(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

func (c Config) SecretsConfigured() bool {
	return len(c.MasterKey) == 32
}
