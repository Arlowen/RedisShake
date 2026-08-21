package engine

import (
	"encoding/json"
	"errors"
	"time"

	"RedisShake/internal/controlplane/domain"
)

var (
	ErrWorkerUnavailable = errors.New("RedisShake worker is unavailable")
	ErrWorkerStart       = errors.New("RedisShake worker failed to start")
	ErrRunNotManaged     = errors.New("run is not managed by this control-plane process")
)

type StartRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type RunView struct {
	ID                  string          `json:"id"`
	TaskID              string          `json:"task_id"`
	ConfigRevision      int64           `json:"config_revision"`
	ConfigSnapshot      json.RawMessage `json:"config_snapshot"`
	State               domain.RunState `json:"state"`
	PID                 *int            `json:"pid,omitempty"`
	StatusPort          *int            `json:"status_port,omitempty"`
	StartedAt           time.Time       `json:"started_at"`
	FinishedAt          *time.Time      `json:"finished_at,omitempty"`
	ExitCode            *int            `json:"exit_code,omitempty"`
	ExitReason          string          `json:"exit_reason,omitempty"`
	LastHeartbeatAt     *time.Time      `json:"last_heartbeat_at,omitempty"`
	StopRequestedByUser bool            `json:"stop_requested_by_user"`
	StatusHealthy       bool            `json:"status_healthy"`
	Status              json.RawMessage `json:"status,omitempty"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type LogResult struct {
	Content    string `json:"content"`
	NextOffset int64  `json:"next_offset"`
	EOF        bool   `json:"eof"`
}

func toRunView(run domain.Run) RunView {
	view := RunView{
		ID:                  run.ID,
		TaskID:              run.TaskID,
		ConfigRevision:      run.ConfigRevision,
		State:               run.State,
		PID:                 run.PID,
		StatusPort:          run.StatusPort,
		StartedAt:           run.StartedAt,
		FinishedAt:          run.FinishedAt,
		ExitCode:            run.ExitCode,
		ExitReason:          run.ExitReason,
		LastHeartbeatAt:     run.LastHeartbeatAt,
		StopRequestedByUser: run.StopRequestedByUser,
		StatusHealthy:       run.StatusHealthy,
		UpdatedAt:           run.UpdatedAt,
	}
	if json.Valid([]byte(run.ConfigSnapshotJSON)) {
		view.ConfigSnapshot = json.RawMessage(run.ConfigSnapshotJSON)
	}
	if json.Valid([]byte(run.StatusJSON)) {
		view.Status = json.RawMessage(run.StatusJSON)
	}
	return view
}
