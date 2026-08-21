package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"RedisShake/internal/controlplane/domain"
)

const timeLayout = time.RFC3339Nano

func (s *Store) CreateConnection(ctx context.Context, connection domain.Connection) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO connections (
		id, name, topology, address, username, password_ciphertext,
		tls_enabled, tls_config_json, sentinel_master_name,
		created_at, updated_at, last_tested_at, last_test_result_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		connection.ID,
		connection.Name,
		connection.Topology,
		connection.Address,
		connection.Username,
		connection.PasswordCiphertext,
		boolToInt(connection.TLSEnabled),
		emptyJSON(connection.TLSConfigJSON),
		connection.SentinelMasterName,
		formatTime(connection.CreatedAt),
		formatTime(connection.UpdatedAt),
		formatOptionalTime(connection.LastTestedAt),
		connection.LastTestResultJSON,
	)
	if err != nil {
		return fmt.Errorf("create connection: %w", err)
	}
	return nil
}

func (s *Store) GetConnection(ctx context.Context, id string) (domain.Connection, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, name, topology, address, username, password_ciphertext,
		tls_enabled, tls_config_json, sentinel_master_name,
		created_at, updated_at, last_tested_at, last_test_result_json
		FROM connections WHERE id = ?`, id)
	return scanConnection(row)
}

func (s *Store) ListConnections(ctx context.Context) ([]domain.Connection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, name, topology, address, username, password_ciphertext,
		tls_enabled, tls_config_json, sentinel_master_name,
		created_at, updated_at, last_tested_at, last_test_result_json
		FROM connections ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	connections := make([]domain.Connection, 0)
	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connections: %w", err)
	}
	return connections, nil
}

func (s *Store) CreateTask(ctx context.Context, task domain.Task) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tasks (
		id, name, description, mode, source_connection_id, target_connection_id,
		reader_options_json, filter_options_json, advanced_options_json,
		state, config_revision, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID,
		task.Name,
		task.Description,
		task.Mode,
		task.SourceConnectionID,
		task.TargetConnectionID,
		emptyJSON(task.ReaderOptionsJSON),
		emptyJSON(task.FilterOptionsJSON),
		emptyJSON(task.AdvancedOptionsJSON),
		task.State,
		task.ConfigRevision,
		formatTime(task.CreatedAt),
		formatTime(task.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (s *Store) GetTask(ctx context.Context, id string) (domain.Task, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, name, description, mode, source_connection_id, target_connection_id,
		reader_options_json, filter_options_json, advanced_options_json,
		state, config_revision, created_at, updated_at
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (s *Store) ListTasks(ctx context.Context) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, name, description, mode, source_connection_id, target_connection_id,
		reader_options_json, filter_options_json, advanced_options_json,
		state, config_revision, created_at, updated_at
		FROM tasks ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, nil
}

func (s *Store) CreateRun(ctx context.Context, run domain.Run) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs (
		id, task_id, config_revision, config_snapshot_json, state,
		pid, process_started_at, status_port, runtime_dir, started_at,
		finished_at, exit_code, exit_reason, last_heartbeat_at, stop_requested_by_user
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID,
		run.TaskID,
		run.ConfigRevision,
		emptyJSON(run.ConfigSnapshotJSON),
		run.State,
		optionalInt(run.PID),
		formatOptionalTime(run.ProcessStartedAt),
		optionalInt(run.StatusPort),
		run.RuntimeDir,
		formatTime(run.StartedAt),
		formatOptionalTime(run.FinishedAt),
		optionalInt(run.ExitCode),
		run.ExitReason,
		formatOptionalTime(run.LastHeartbeatAt),
		boolToInt(run.StopRequestedByUser),
	)
	if err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, id string) (domain.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, task_id, config_revision, config_snapshot_json, state,
		pid, process_started_at, status_port, runtime_dir, started_at,
		finished_at, exit_code, exit_reason, last_heartbeat_at, stop_requested_by_user
		FROM runs WHERE id = ?`, id)
	return scanRun(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConnection(row scanner) (domain.Connection, error) {
	var connection domain.Connection
	var tlsEnabled int
	var createdAt, updatedAt string
	var lastTestedAt sql.NullString
	err := row.Scan(
		&connection.ID,
		&connection.Name,
		&connection.Topology,
		&connection.Address,
		&connection.Username,
		&connection.PasswordCiphertext,
		&tlsEnabled,
		&connection.TLSConfigJSON,
		&connection.SentinelMasterName,
		&createdAt,
		&updatedAt,
		&lastTestedAt,
		&connection.LastTestResultJSON,
	)
	if err != nil {
		return domain.Connection{}, mapNotFound(err)
	}
	connection.TLSEnabled = tlsEnabled == 1
	if connection.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Connection{}, err
	}
	if connection.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Connection{}, err
	}
	if connection.LastTestedAt, err = parseOptionalTime(lastTestedAt); err != nil {
		return domain.Connection{}, err
	}
	return connection, nil
}

func scanTask(row scanner) (domain.Task, error) {
	var task domain.Task
	var createdAt, updatedAt string
	err := row.Scan(
		&task.ID,
		&task.Name,
		&task.Description,
		&task.Mode,
		&task.SourceConnectionID,
		&task.TargetConnectionID,
		&task.ReaderOptionsJSON,
		&task.FilterOptionsJSON,
		&task.AdvancedOptionsJSON,
		&task.State,
		&task.ConfigRevision,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.Task{}, mapNotFound(err)
	}
	if task.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Task{}, err
	}
	if task.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Task{}, err
	}
	return task, nil
}

func scanRun(row scanner) (domain.Run, error) {
	var run domain.Run
	var pid, statusPort, exitCode sql.NullInt64
	var processStartedAt, finishedAt, heartbeatAt sql.NullString
	var startedAt string
	var stopRequested int
	err := row.Scan(
		&run.ID,
		&run.TaskID,
		&run.ConfigRevision,
		&run.ConfigSnapshotJSON,
		&run.State,
		&pid,
		&processStartedAt,
		&statusPort,
		&run.RuntimeDir,
		&startedAt,
		&finishedAt,
		&exitCode,
		&run.ExitReason,
		&heartbeatAt,
		&stopRequested,
	)
	if err != nil {
		return domain.Run{}, mapNotFound(err)
	}
	run.PID = nullIntToPointer(pid)
	run.StatusPort = nullIntToPointer(statusPort)
	run.ExitCode = nullIntToPointer(exitCode)
	run.StopRequestedByUser = stopRequested == 1
	if run.StartedAt, err = parseTime(startedAt); err != nil {
		return domain.Run{}, err
	}
	if run.ProcessStartedAt, err = parseOptionalTime(processStartedAt); err != nil {
		return domain.Run{}, err
	}
	if run.FinishedAt, err = parseOptionalTime(finishedAt); err != nil {
		return domain.Run{}, err
	}
	if run.LastHeartbeatAt, err = parseOptionalTime(heartbeatAt); err != nil {
		return domain.Run{}, err
	}
	return run, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(timeLayout)
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(timeLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored time %q: %w", value, err)
	}
	return parsed, nil
}

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func optionalInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullIntToPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func emptyJSON(value string) string {
	if value == "" {
		return "{}"
	}
	return value
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
