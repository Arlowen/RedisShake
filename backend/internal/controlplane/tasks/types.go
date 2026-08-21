package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"RedisShake/internal/controlplane/connections"
	"RedisShake/internal/controlplane/domain"
)

var ErrArchived = errors.New("task is archived")

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type SyncReaderOptions struct {
	SyncRDB       bool `json:"sync_rdb"`
	SyncAOF       bool `json:"sync_aof"`
	PreferReplica bool `json:"prefer_replica"`
	TryDiskless   bool `json:"try_diskless"`
}

type ScanReaderOptions struct {
	DBs             []int    `json:"dbs"`
	Scan            bool     `json:"scan"`
	KSN             bool     `json:"ksn"`
	Count           int      `json:"count"`
	PreferReplica   bool     `json:"prefer_replica"`
	SkipUnknownType []string `json:"skip_unknown_type"`
}

type FilterOptions struct {
	AllowKeys         []string `json:"allow_keys"`
	AllowKeyPrefix    []string `json:"allow_key_prefix"`
	AllowKeySuffix    []string `json:"allow_key_suffix"`
	AllowKeyRegex     []string `json:"allow_key_regex"`
	BlockKeys         []string `json:"block_keys"`
	BlockKeyPrefix    []string `json:"block_key_prefix"`
	BlockKeySuffix    []string `json:"block_key_suffix"`
	BlockKeyRegex     []string `json:"block_key_regex"`
	AllowDB           []int    `json:"allow_db"`
	BlockDB           []int    `json:"block_db"`
	AllowCommand      []string `json:"allow_command"`
	BlockCommand      []string `json:"block_command"`
	AllowCommandGroup []string `json:"allow_command_group"`
	BlockCommandGroup []string `json:"block_command_group"`
	Function          string   `json:"function"`
}

type AdvancedOptions struct {
	RDBRestoreCommandBehavior    string `json:"rdb_restore_command_behavior"`
	PipelineCountLimit           uint64 `json:"pipeline_count_limit"`
	TargetRedisMaxQPS            int    `json:"target_redis_max_qps"`
	TargetRedisClientMaxQuerybuf int64  `json:"target_redis_client_max_querybuf_len"`
	TargetRedisProtoMaxBulkLen   uint64 `json:"target_redis_proto_max_bulk_len"`
	EmptyDBBeforeSync            bool   `json:"empty_db_before_sync"`
}

type Spec struct {
	Name               string             `json:"name"`
	Description        string             `json:"description,omitempty"`
	Mode               domain.TaskMode    `json:"mode"`
	SourceConnectionID string             `json:"source_connection_id,omitempty"`
	TargetConnectionID string             `json:"target_connection_id,omitempty"`
	SyncReader         *SyncReaderOptions `json:"sync_reader,omitempty"`
	ScanReader         *ScanReaderOptions `json:"scan_reader,omitempty"`
	Filter             FilterOptions      `json:"filter"`
	Advanced           AdvancedOptions    `json:"advanced"`
}

type Patch struct {
	ExpectedRevision   int64              `json:"expected_revision"`
	Name               *string            `json:"name,omitempty"`
	Description        *string            `json:"description,omitempty"`
	Mode               *domain.TaskMode   `json:"mode,omitempty"`
	SourceConnectionID *string            `json:"source_connection_id,omitempty"`
	TargetConnectionID *string            `json:"target_connection_id,omitempty"`
	SyncReader         *SyncReaderOptions `json:"sync_reader,omitempty"`
	ScanReader         *ScanReaderOptions `json:"scan_reader,omitempty"`
	Filter             *FilterOptions     `json:"filter,omitempty"`
	Advanced           *AdvancedOptions   `json:"advanced,omitempty"`
}

type View struct {
	ID                 string           `json:"id"`
	Spec               Spec             `json:"spec"`
	State              domain.TaskState `json:"state"`
	ConfigRevision     int64            `json:"config_revision"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
	LastPrecheckedAt   *time.Time       `json:"last_prechecked_at,omitempty"`
	LastPrecheckResult json.RawMessage  `json:"last_precheck_result,omitempty"`
}

type PrecheckRequest struct {
	ExpectedRevision    int64 `json:"expected_revision"`
	AcknowledgeWarnings bool  `json:"acknowledge_warnings"`
}

type PrecheckResult struct {
	TaskID         string                  `json:"task_id"`
	ConfigRevision int64                   `json:"config_revision"`
	Ready          bool                    `json:"ready"`
	ConfigDigest   string                  `json:"config_digest,omitempty"`
	Checks         []connections.CheckItem `json:"checks"`
	CheckedAt      time.Time               `json:"checked_at"`
}

type RuntimeConfig struct {
	RunDir     string
	StatusPort int
}

type Artifact struct {
	TOML           []byte
	DigestMaterial []byte
	SecretFiles    map[string][]byte
}

type Renderer interface {
	Render(spec Spec, source, target connections.Resolved, runtime RuntimeConfig) (Artifact, error)
}
