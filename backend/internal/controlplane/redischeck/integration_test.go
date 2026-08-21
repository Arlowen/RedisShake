package redischeck

import (
	"context"
	"os"
	"testing"
	"time"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
)

func TestRealRedisConnection(t *testing.T) {
	address := os.Getenv("REDISSHAKE_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("REDISSHAKE_TEST_REDIS_ADDR is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := (&Checker{Timeout: 5 * time.Second}).Check(ctx, connections.Resolved{Spec: connections.Spec{
		Name:     "Integration Redis",
		Topology: domain.TopologyStandalone,
		Address:  address,
		Username: os.Getenv("REDISSHAKE_TEST_REDIS_USERNAME"),
		Password: os.Getenv("REDISSHAKE_TEST_REDIS_PASSWORD"),
	}}, connections.TestPurposeTarget)
	if !result.Success {
		t.Fatalf("real Redis connection test failed: %+v", result)
	}
	if !hasCheck(result, "target_write", connections.CheckStatePass) || !hasCheck(result, "target_cleanup", connections.CheckStatePass) {
		t.Fatalf("real Redis target write checks missing: %+v", result)
	}
}
