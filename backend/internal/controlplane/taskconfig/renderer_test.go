package taskconfig

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	redisconfig "RedisShake/internal/config"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/tasks"
	"RedisShake/internal/reader"
)

func TestRenderSyncConfigIsAcceptedByRedisShake(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	artifact, err := (&Renderer{}).Render(tasks.Spec{
		Name:       "Sync task",
		Mode:       domain.TaskModeSync,
		SyncReader: &tasks.SyncReaderOptions{SyncRDB: true, SyncAOF: true},
		Filter: tasks.FilterOptions{
			BlockKeyPrefix: []string{"cache:"},
		},
		Advanced: testAdvancedOptions(),
	}, connections.Resolved{Spec: connections.Spec{
		Topology: domain.TopologyStandalone,
		Address:  "127.0.0.1:6379",
		Username: "source-user",
		Password: "source-\"password",
		TLS: connections.TLSConfig{
			Enabled:            true,
			ServerName:         "redis.source.internal",
			InsecureSkipVerify: false,
			CACertPEM:          "source-ca-pem",
		},
	}}, connections.Resolved{Spec: connections.Spec{
		Topology: domain.TopologyStandalone,
		Address:  "127.0.0.1:6380",
		Username: "target-user",
		Password: "target-password",
	}}, tasks.RuntimeConfig{RunDir: runDir, StatusPort: 19001})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if bytes.Contains(artifact.TOML, []byte("source-ca-pem")) {
		t.Fatal("TOML contains inline certificate material")
	}
	if !bytes.Contains(artifact.TOML, []byte(`source-"password`)) || !bytes.Contains(artifact.TOML, []byte("target-password")) {
		t.Fatal("runtime TOML is missing Redis credentials")
	}
	if bytes.Contains(artifact.DigestMaterial, []byte(`source-"password`)) || bytes.Contains(artifact.DigestMaterial, []byte("target-password")) {
		t.Fatal("configuration digest material contains Redis credentials")
	}
	caPath := filepath.Join(runDir, "certs", "source-ca.pem")
	if string(artifact.SecretFiles[caPath]) != "source-ca-pem" {
		t.Fatalf("SecretFiles[%q] missing", caPath)
	}
	v, options, err := redisconfig.ParseConfigBytes(artifact.TOML)
	if err != nil {
		t.Fatalf("ParseConfigBytes() error = %v\n%s", err, artifact.TOML)
	}
	if err := redisconfig.ValidateConfigSections(v); err != nil {
		t.Fatalf("ValidateConfigSections() error = %v", err)
	}
	if v.GetString("sync_reader.address") != "127.0.0.1:6379" || v.GetString("redis_writer.address") != "127.0.0.1:6380" {
		t.Fatalf("rendered endpoints missing:\n%s", artifact.TOML)
	}
	if v.GetString("sync_reader.password") != `source-"password` {
		t.Fatalf("rendered password was not TOML escaped correctly")
	}
	if options.Advanced.StatusPort != 19001 || options.Advanced.Dir != filepath.Join(runDir, "data") {
		t.Fatalf("advanced options = %+v", options.Advanced)
	}
	if options.Advanced.TargetRedisMaxQPS != 1000 {
		t.Fatalf("TargetRedisMaxQPS = %d", options.Advanced.TargetRedisMaxQPS)
	}
	if !v.IsSet("sync_reader.tls_config.insecure_skip_verify") || v.GetBool("sync_reader.tls_config.insecure_skip_verify") {
		t.Fatal("generated TLS config did not explicitly enable certificate verification")
	}
	parsedReader := reader.SyncReaderOptions{}
	if err := v.UnmarshalKey("sync_reader", &parsedReader); err != nil {
		t.Fatalf("UnmarshalKey(sync_reader) error = %v", err)
	}
	if parsedReader.TlsConfig.InsecureSkipVerify == nil || *parsedReader.TlsConfig.InsecureSkipVerify {
		t.Fatal("RedisShake reader options did not preserve insecure_skip_verify=false")
	}
}

func TestRenderScanSentinelConfig(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	artifact, err := (&Renderer{}).Render(tasks.Spec{
		Name:       "Scan task",
		Mode:       domain.TaskModeScan,
		ScanReader: &tasks.ScanReaderOptions{DBs: []int{0, 1}, Scan: true, Count: 100},
		Advanced:   testAdvancedOptions(),
	}, connections.Resolved{Spec: connections.Spec{
		Topology: domain.TopologySentinel,
		Username: "redis-user",
		Password: "redis-password",
		Sentinel: connections.SentinelConfig{
			Address:    "127.0.0.1:26379",
			MasterName: "mymaster",
			Username:   "sentinel-user",
			Password:   "sentinel-password",
		},
	}}, connections.Resolved{Spec: connections.Spec{
		Topology: domain.TopologyCluster,
		Address:  "127.0.0.1:7000",
	}}, tasks.RuntimeConfig{RunDir: runDir})
	if err != nil {
		t.Fatalf("Render() error = %v\n%s", err, artifact.TOML)
	}
	text := string(artifact.TOML)
	for _, expected := range []string{"[scan_reader]", "[scan_reader.sentinel]", `master_name = 'mymaster'`, "[redis_writer]", "cluster = true"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("rendered TOML missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "[sync_reader]") {
		t.Fatalf("scan config contains sync_reader:\n%s", text)
	}
}

func testAdvancedOptions() tasks.AdvancedOptions {
	return tasks.AdvancedOptions{
		RDBRestoreCommandBehavior:    "panic",
		PipelineCountLimit:           1024,
		TargetRedisMaxQPS:            1000,
		TargetRedisClientMaxQuerybuf: 1073741824,
		TargetRedisProtoMaxBulkLen:   512000000,
	}
}
