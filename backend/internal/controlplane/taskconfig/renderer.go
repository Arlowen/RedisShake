package taskconfig

import (
	"errors"
	"fmt"
	"path/filepath"

	redisconfig "RedisShake/internal/config"
	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
	"RedisShake/internal/controlplane/tasks"

	"github.com/pelletier/go-toml/v2"
)

type Renderer struct{}

type document struct {
	SyncReader  *syncReaderDocument `toml:"sync_reader,omitempty"`
	ScanReader  *scanReaderDocument `toml:"scan_reader,omitempty"`
	RedisWriter endpointDocument    `toml:"redis_writer"`
	Filter      filterDocument      `toml:"filter"`
	Advanced    advancedDocument    `toml:"advanced"`
}

type endpointDocument struct {
	Cluster   bool              `toml:"cluster"`
	Address   string            `toml:"address"`
	Username  string            `toml:"username"`
	Password  string            `toml:"password"`
	TLS       bool              `toml:"tls"`
	TLSConfig *tlsDocument      `toml:"tls_config,omitempty"`
	Sentinel  *sentinelDocument `toml:"sentinel,omitempty"`
	OffReply  bool              `toml:"off_reply,omitempty"`
}

type sentinelDocument struct {
	MasterName string       `toml:"master_name"`
	Address    string       `toml:"address"`
	Username   string       `toml:"username"`
	Password   string       `toml:"password"`
	TLS        bool         `toml:"tls"`
	TLSConfig  *tlsDocument `toml:"tls_config,omitempty"`
}

type tlsDocument struct {
	CACert             string `toml:"ca_cert,omitempty"`
	Cert               string `toml:"cert,omitempty"`
	Key                string `toml:"key,omitempty"`
	ServerName         string `toml:"server_name,omitempty"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
}

type syncReaderDocument struct {
	Cluster       bool              `toml:"cluster"`
	Address       string            `toml:"address"`
	Username      string            `toml:"username"`
	Password      string            `toml:"password"`
	TLS           bool              `toml:"tls"`
	TLSConfig     *tlsDocument      `toml:"tls_config,omitempty"`
	Sentinel      *sentinelDocument `toml:"sentinel,omitempty"`
	SyncRDB       bool              `toml:"sync_rdb"`
	SyncAOF       bool              `toml:"sync_aof"`
	PreferReplica bool              `toml:"prefer_replica"`
	TryDiskless   bool              `toml:"try_diskless"`
}

type scanReaderDocument struct {
	Cluster         bool              `toml:"cluster"`
	Address         string            `toml:"address"`
	Username        string            `toml:"username"`
	Password        string            `toml:"password"`
	TLS             bool              `toml:"tls"`
	TLSConfig       *tlsDocument      `toml:"tls_config,omitempty"`
	Sentinel        *sentinelDocument `toml:"sentinel,omitempty"`
	DBs             []int             `toml:"dbs"`
	Scan            bool              `toml:"scan"`
	KSN             bool              `toml:"ksn"`
	Count           int               `toml:"count"`
	PreferReplica   bool              `toml:"prefer_replica"`
	SkipUnknownType []string          `toml:"skip_unknown_type"`
}

type filterDocument struct {
	AllowKeys         []string `toml:"allow_keys"`
	AllowKeyPrefix    []string `toml:"allow_key_prefix"`
	AllowKeySuffix    []string `toml:"allow_key_suffix"`
	AllowKeyRegex     []string `toml:"allow_key_regex"`
	BlockKeys         []string `toml:"block_keys"`
	BlockKeyPrefix    []string `toml:"block_key_prefix"`
	BlockKeySuffix    []string `toml:"block_key_suffix"`
	BlockKeyRegex     []string `toml:"block_key_regex"`
	AllowDB           []int    `toml:"allow_db"`
	BlockDB           []int    `toml:"block_db"`
	AllowCommand      []string `toml:"allow_command"`
	BlockCommand      []string `toml:"block_command"`
	AllowCommandGroup []string `toml:"allow_command_group"`
	BlockCommandGroup []string `toml:"block_command_group"`
	Function          string   `toml:"function"`
}

type advancedDocument struct {
	Dir                             string `toml:"dir"`
	NCPU                            int    `toml:"ncpu"`
	PProfPort                       int    `toml:"pprof_port"`
	StatusPort                      int    `toml:"status_port"`
	LogFile                         string `toml:"log_file"`
	LogLevel                        string `toml:"log_level"`
	LogInterval                     int    `toml:"log_interval"`
	LogRotation                     bool   `toml:"log_rotation"`
	LogMaxSize                      int    `toml:"log_max_size"`
	LogMaxAge                       int    `toml:"log_max_age"`
	LogMaxBackups                   int    `toml:"log_max_backups"`
	LogCompress                     bool   `toml:"log_compress"`
	RDBRestoreCommandBehavior       string `toml:"rdb_restore_command_behavior"`
	PipelineCountLimit              uint64 `toml:"pipeline_count_limit"`
	TargetRedisMaxQPS               int    `toml:"target_redis_max_qps"`
	TargetRedisClientMaxQuerybufLen int64  `toml:"target_redis_client_max_querybuf_len"`
	TargetRedisProtoMaxBulkLen      uint64 `toml:"target_redis_proto_max_bulk_len"`
	EmptyDBBeforeSync               bool   `toml:"empty_db_before_sync"`
}

func (r *Renderer) Render(spec tasks.Spec, source, target connections.Resolved, runtime tasks.RuntimeConfig) (tasks.Artifact, error) {
	if runtime.RunDir == "" {
		return tasks.Artifact{}, errors.New("runtime directory is required")
	}
	if runtime.StatusPort < 0 || runtime.StatusPort > 65535 {
		return tasks.Artifact{}, fmt.Errorf("invalid status port %d", runtime.StatusPort)
	}
	artifact := tasks.Artifact{SecretFiles: make(map[string][]byte)}
	sourceEndpoint := buildEndpoint(source, "source", runtime.RunDir, &artifact)
	targetEndpoint := buildEndpoint(target, "target", runtime.RunDir, &artifact)
	targetEndpoint.OffReply = false
	document := document{
		RedisWriter: targetEndpoint,
		Filter:      buildFilter(spec.Filter),
		Advanced: advancedDocument{
			Dir:                             filepath.Join(runtime.RunDir, "data"),
			NCPU:                            0,
			PProfPort:                       0,
			StatusPort:                      runtime.StatusPort,
			LogFile:                         "shake.log",
			LogLevel:                        "info",
			LogInterval:                     5,
			LogRotation:                     true,
			LogMaxSize:                      512,
			LogMaxAge:                       7,
			LogMaxBackups:                   3,
			LogCompress:                     true,
			RDBRestoreCommandBehavior:       spec.Advanced.RDBRestoreCommandBehavior,
			PipelineCountLimit:              spec.Advanced.PipelineCountLimit,
			TargetRedisMaxQPS:               spec.Advanced.TargetRedisMaxQPS,
			TargetRedisClientMaxQuerybufLen: spec.Advanced.TargetRedisClientMaxQuerybuf,
			TargetRedisProtoMaxBulkLen:      spec.Advanced.TargetRedisProtoMaxBulkLen,
			EmptyDBBeforeSync:               spec.Advanced.EmptyDBBeforeSync,
		},
	}
	switch spec.Mode {
	case domain.TaskModeSync:
		if spec.SyncReader == nil {
			return tasks.Artifact{}, errors.New("sync reader options are required")
		}
		document.SyncReader = &syncReaderDocument{
			Cluster:       sourceEndpoint.Cluster,
			Address:       sourceEndpoint.Address,
			Username:      sourceEndpoint.Username,
			Password:      sourceEndpoint.Password,
			TLS:           sourceEndpoint.TLS,
			TLSConfig:     sourceEndpoint.TLSConfig,
			Sentinel:      sourceEndpoint.Sentinel,
			SyncRDB:       spec.SyncReader.SyncRDB,
			SyncAOF:       spec.SyncReader.SyncAOF,
			PreferReplica: spec.SyncReader.PreferReplica,
			TryDiskless:   spec.SyncReader.TryDiskless,
		}
	case domain.TaskModeScan:
		if spec.ScanReader == nil {
			return tasks.Artifact{}, errors.New("scan reader options are required")
		}
		document.ScanReader = &scanReaderDocument{
			Cluster:         sourceEndpoint.Cluster,
			Address:         sourceEndpoint.Address,
			Username:        sourceEndpoint.Username,
			Password:        sourceEndpoint.Password,
			TLS:             sourceEndpoint.TLS,
			TLSConfig:       sourceEndpoint.TLSConfig,
			Sentinel:        sourceEndpoint.Sentinel,
			DBs:             nonNilInts(spec.ScanReader.DBs),
			Scan:            spec.ScanReader.Scan,
			KSN:             spec.ScanReader.KSN,
			Count:           spec.ScanReader.Count,
			PreferReplica:   spec.ScanReader.PreferReplica,
			SkipUnknownType: nonNilStrings(spec.ScanReader.SkipUnknownType),
		}
	default:
		return tasks.Artifact{}, fmt.Errorf("unsupported task mode %q", spec.Mode)
	}

	encoded, err := toml.Marshal(document)
	if err != nil {
		return tasks.Artifact{}, fmt.Errorf("encode RedisShake TOML: %w", err)
	}
	v, _, err := redisconfig.ParseConfigBytes(encoded)
	if err != nil {
		return tasks.Artifact{}, err
	}
	if err := redisconfig.ValidateConfigSections(v); err != nil {
		return tasks.Artifact{}, err
	}
	digestMaterial, err := toml.Marshal(redactCredentials(document))
	if err != nil {
		return tasks.Artifact{}, fmt.Errorf("encode sanitized RedisShake TOML: %w", err)
	}
	artifact.TOML = encoded
	artifact.DigestMaterial = digestMaterial
	return artifact, nil
}

func redactCredentials(value document) document {
	value.RedisWriter.Password = ""
	value.RedisWriter.Sentinel = redactSentinel(value.RedisWriter.Sentinel)
	if value.SyncReader != nil {
		copy := *value.SyncReader
		copy.Password = ""
		copy.Sentinel = redactSentinel(copy.Sentinel)
		value.SyncReader = &copy
	}
	if value.ScanReader != nil {
		copy := *value.ScanReader
		copy.Password = ""
		copy.Sentinel = redactSentinel(copy.Sentinel)
		value.ScanReader = &copy
	}
	return value
}

func redactSentinel(value *sentinelDocument) *sentinelDocument {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Password = ""
	return &copy
}

func buildEndpoint(connection connections.Resolved, prefix, runDir string, artifact *tasks.Artifact) endpointDocument {
	endpoint := endpointDocument{
		Cluster:  connection.Topology == domain.TopologyCluster,
		Address:  connection.Address,
		Username: connection.Username,
		Password: connection.Password,
		TLS:      connection.TLS.Enabled,
	}
	endpoint.TLSConfig = buildTLS(connection.TLS, prefix, runDir, artifact)
	if connection.Topology == domain.TopologySentinel {
		endpoint.Sentinel = &sentinelDocument{
			MasterName: connection.Sentinel.MasterName,
			Address:    connection.Sentinel.Address,
			Username:   connection.Sentinel.Username,
			Password:   connection.Sentinel.Password,
			TLS:        connection.Sentinel.TLS.Enabled,
			TLSConfig:  buildTLS(connection.Sentinel.TLS, prefix+"-sentinel", runDir, artifact),
		}
	}
	return endpoint
}

func buildTLS(config connections.TLSConfig, prefix, runDir string, artifact *tasks.Artifact) *tlsDocument {
	if !config.Enabled {
		return nil
	}
	document := &tlsDocument{
		ServerName:         config.ServerName,
		InsecureSkipVerify: config.InsecureSkipVerify,
	}
	certDir := filepath.Join(runDir, "certs")
	if config.CACertPEM != "" {
		document.CACert = filepath.Join(certDir, prefix+"-ca.pem")
		artifact.SecretFiles[document.CACert] = []byte(config.CACertPEM)
	}
	if config.ClientCertPEM != "" {
		document.Cert = filepath.Join(certDir, prefix+"-client-cert.pem")
		artifact.SecretFiles[document.Cert] = []byte(config.ClientCertPEM)
	}
	if config.ClientKeyPEM != "" {
		document.Key = filepath.Join(certDir, prefix+"-client-key.pem")
		artifact.SecretFiles[document.Key] = []byte(config.ClientKeyPEM)
	}
	return document
}

func buildFilter(config tasks.FilterOptions) filterDocument {
	return filterDocument{
		AllowKeys:         nonNilStrings(config.AllowKeys),
		AllowKeyPrefix:    nonNilStrings(config.AllowKeyPrefix),
		AllowKeySuffix:    nonNilStrings(config.AllowKeySuffix),
		AllowKeyRegex:     nonNilStrings(config.AllowKeyRegex),
		BlockKeys:         nonNilStrings(config.BlockKeys),
		BlockKeyPrefix:    nonNilStrings(config.BlockKeyPrefix),
		BlockKeySuffix:    nonNilStrings(config.BlockKeySuffix),
		BlockKeyRegex:     nonNilStrings(config.BlockKeyRegex),
		AllowDB:           nonNilInts(config.AllowDB),
		BlockDB:           nonNilInts(config.BlockDB),
		AllowCommand:      nonNilStrings(config.AllowCommand),
		BlockCommand:      nonNilStrings(config.BlockCommand),
		AllowCommandGroup: nonNilStrings(config.AllowCommandGroup),
		BlockCommandGroup: nonNilStrings(config.BlockCommandGroup),
		Function:          config.Function,
	}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilInts(values []int) []int {
	if values == nil {
		return []int{}
	}
	return values
}
