package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"RedisShake/internal/controlplane/api"
	cpconfig "RedisShake/internal/controlplane/config"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/redischeck"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
)

func main() {
	if err := run(); err != nil {
		log.Printf("redis-shake-server failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	config, err := cpconfig.Load()
	if err != nil {
		return fmt.Errorf("load control plane configuration: %w", err)
	}

	ctx := context.Background()
	database, err := store.Open(ctx, config.DatabasePath)
	if err != nil {
		return err
	}
	defer database.Close()

	secretCipher, err := prepareSecretCipher(ctx, database, config.MasterKey)
	if err != nil {
		return err
	}
	connectionService := connections.NewService(database, secretCipher, &redischeck.Checker{Timeout: 5 * time.Second})
	if err := connectionService.ValidateStoredSecrets(ctx); err != nil {
		return err
	}

	apiServer := api.NewServer(database, config, api.BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
	}, connectionService)
	httpServer := &http.Server{
		Addr:              config.ListenAddress,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errChannel := make(chan error, 1)
	go func() {
		log.Printf("redis-shake-server %s (%s) listening on %s", Version, GitCommit, config.ListenAddress)
		errChannel <- httpServer.ListenAndServe()
	}()

	select {
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	case err := <-errChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	}
}

func prepareSecretCipher(ctx context.Context, database *store.Store, key []byte) (*secrets.Cipher, error) {
	hasSecrets, err := database.HasEncryptedSecrets(ctx)
	if err != nil {
		return nil, err
	}
	if len(key) == 0 {
		if hasSecrets {
			return nil, errors.New("REDISSHAKE_MASTER_KEY is required because encrypted connection credentials already exist")
		}
		return nil, nil
	}
	cipher, err := secrets.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher, nil
}
