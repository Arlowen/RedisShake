package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/secrets"
	"RedisShake/internal/controlplane/store"
)

func TestValidateStoredSecrets(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "control-plane.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer database.Close()

	key := bytes.Repeat([]byte{0x41}, 32)
	cipher, err := secrets.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	ciphertext, err := cipher.Encrypt("credential")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	now := time.Now().UTC()
	if err := database.CreateConnection(ctx, domain.Connection{
		ID:                 "connection-1",
		Name:               "Protected connection",
		Topology:           domain.TopologyStandalone,
		Address:            "127.0.0.1:6379",
		PasswordCiphertext: ciphertext,
		TLSConfigJSON:      `{}`,
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("CreateConnection() error = %v", err)
	}

	if err := validateStoredSecrets(ctx, database, nil); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("validateStoredSecrets(nil) error = %v", err)
	}
	if err := validateStoredSecrets(ctx, database, bytes.Repeat([]byte{0x42}, 32)); err == nil || !strings.Contains(err.Error(), "cannot decrypt") {
		t.Fatalf("validateStoredSecrets(wrong key) error = %v", err)
	}
	if err := validateStoredSecrets(ctx, database, key); err != nil {
		t.Fatalf("validateStoredSecrets(correct key) error = %v", err)
	}
}
