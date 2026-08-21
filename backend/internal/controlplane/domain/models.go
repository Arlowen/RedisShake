package domain

import (
	"fmt"
	"time"
)

type Topology string

const (
	TopologyStandalone Topology = "standalone"
	TopologySentinel   Topology = "sentinel"
	TopologyCluster    Topology = "cluster"
)

func (t Topology) Valid() bool {
	return t == TopologyStandalone || t == TopologySentinel || t == TopologyCluster
}

type Connection struct {
	ID                         string     `json:"id"`
	Name                       string     `json:"name"`
	Topology                   Topology   `json:"topology"`
	Address                    string     `json:"address"`
	Username                   string     `json:"username,omitempty"`
	PasswordCiphertext         string     `json:"-"`
	TLSEnabled                 bool       `json:"tls_enabled"`
	TLSConfigJSON              string     `json:"-"`
	SentinelAddress            string     `json:"sentinel_address,omitempty"`
	SentinelMasterName         string     `json:"sentinel_master_name,omitempty"`
	SentinelUsername           string     `json:"sentinel_username,omitempty"`
	SentinelPasswordCiphertext string     `json:"-"`
	SentinelTLSEnabled         bool       `json:"sentinel_tls_enabled"`
	SentinelTLSConfigJSON      string     `json:"-"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
	LastTestedAt               *time.Time `json:"last_tested_at,omitempty"`
	LastTestResultJSON         string     `json:"-"`
}

type TaskMode string

const (
	TaskModeSync TaskMode = "sync"
	TaskModeScan TaskMode = "scan"
)

func (m TaskMode) Valid() bool {
	return m == TaskModeSync || m == TaskModeScan
}

type TaskState string

const (
	TaskStateDraft    TaskState = "DRAFT"
	TaskStateReady    TaskState = "READY"
	TaskStateArchived TaskState = "ARCHIVED"
)

type Task struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	Description            string     `json:"description,omitempty"`
	Mode                   TaskMode   `json:"mode"`
	SourceConnectionID     string     `json:"source_connection_id"`
	TargetConnectionID     string     `json:"target_connection_id"`
	ReaderOptionsJSON      string     `json:"reader_options"`
	FilterOptionsJSON      string     `json:"filter_options"`
	AdvancedOptionsJSON    string     `json:"advanced_options"`
	State                  TaskState  `json:"state"`
	ConfigRevision         int64      `json:"config_revision"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	LastPrecheckedAt       *time.Time `json:"last_prechecked_at,omitempty"`
	LastPrecheckResultJSON string     `json:"-"`
}

type RunState string

const (
	RunStateStarting  RunState = "STARTING"
	RunStateRunning   RunState = "RUNNING"
	RunStateStopping  RunState = "STOPPING"
	RunStateStopped   RunState = "STOPPED"
	RunStateSucceeded RunState = "SUCCEEDED"
	RunStateFailed    RunState = "FAILED"
	RunStateUnknown   RunState = "UNKNOWN"
)

type Run struct {
	ID                  string     `json:"id"`
	TaskID              string     `json:"task_id"`
	ConfigRevision      int64      `json:"config_revision"`
	ConfigSnapshotJSON  string     `json:"config_snapshot"`
	State               RunState   `json:"state"`
	PID                 *int       `json:"pid,omitempty"`
	ProcessStartedAt    *time.Time `json:"process_started_at,omitempty"`
	StatusPort          *int       `json:"status_port,omitempty"`
	RuntimeDir          string     `json:"runtime_dir"`
	StartedAt           time.Time  `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at,omitempty"`
	ExitCode            *int       `json:"exit_code,omitempty"`
	ExitReason          string     `json:"exit_reason,omitempty"`
	LastHeartbeatAt     *time.Time `json:"last_heartbeat_at,omitempty"`
	StopRequestedByUser bool       `json:"stop_requested_by_user"`
	StatusJSON          string     `json:"-"`
	StatusHealthy       bool       `json:"status_healthy"`
	WorkerPath          string     `json:"worker_path,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func ValidateTaskTransition(from, to TaskState) error {
	if from == to {
		return nil
	}
	allowed := map[TaskState]map[TaskState]bool{
		TaskStateDraft: {
			TaskStateReady:    true,
			TaskStateArchived: true,
		},
		TaskStateReady: {
			TaskStateDraft:    true,
			TaskStateArchived: true,
		},
	}
	if allowed[from][to] {
		return nil
	}
	return fmt.Errorf("invalid task state transition %s -> %s", from, to)
}

func ValidateRunTransition(from, to RunState) error {
	if from == to {
		return nil
	}
	allowed := map[RunState]map[RunState]bool{
		RunStateStarting: {
			RunStateRunning:  true,
			RunStateStopping: true,
			RunStateFailed:   true,
			RunStateUnknown:  true,
		},
		RunStateRunning: {
			RunStateStopping:  true,
			RunStateSucceeded: true,
			RunStateFailed:    true,
			RunStateUnknown:   true,
		},
		RunStateStopping: {
			RunStateStopped: true,
			RunStateFailed:  true,
			RunStateUnknown: true,
		},
		RunStateUnknown: {
			RunStateRunning:  true,
			RunStateStopping: true,
			RunStateStopped:  true,
			RunStateFailed:   true,
		},
	}
	if allowed[from][to] {
		return nil
	}
	return fmt.Errorf("invalid run state transition %s -> %s", from, to)
}
