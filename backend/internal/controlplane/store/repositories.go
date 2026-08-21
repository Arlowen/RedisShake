package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"RedisShake/internal/controlplane/domain"
)

const timeLayout = time.RFC3339Nano

func (s *Store) CreateConnection(ctx context.Context, connection domain.Connection) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO connections (
		id, name, topology, address, username, password_ciphertext,
		tls_enabled, tls_config_json, sentinel_address, sentinel_master_name,
		sentinel_username, sentinel_password_ciphertext, sentinel_tls_enabled, sentinel_tls_config_json,
		created_at, updated_at, last_tested_at, last_test_result_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		connection.ID,
		connection.Name,
		connection.Topology,
		connection.Address,
		connection.Username,
		connection.PasswordCiphertext,
		boolToInt(connection.TLSEnabled),
		emptyJSON(connection.TLSConfigJSON),
		connection.SentinelAddress,
		connection.SentinelMasterName,
		connection.SentinelUsername,
		connection.SentinelPasswordCiphertext,
		boolToInt(connection.SentinelTLSEnabled),
		emptyJSON(connection.SentinelTLSConfigJSON),
		formatTime(connection.CreatedAt),
		formatTime(connection.UpdatedAt),
		formatOptionalTime(connection.LastTestedAt),
		connection.LastTestResultJSON,
	)
	if err != nil {
		return fmt.Errorf("create connection: %w", mapWriteError(err))
	}
	return nil
}

func (s *Store) GetConnection(ctx context.Context, id string) (domain.Connection, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, name, topology, address, username, password_ciphertext,
		tls_enabled, tls_config_json, sentinel_address, sentinel_master_name,
		sentinel_username, sentinel_password_ciphertext, sentinel_tls_enabled, sentinel_tls_config_json,
		created_at, updated_at, last_tested_at, last_test_result_json
		FROM connections WHERE id = ?`, id)
	return scanConnection(row)
}

func (s *Store) ListConnections(ctx context.Context) ([]domain.Connection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, name, topology, address, username, password_ciphertext,
		tls_enabled, tls_config_json, sentinel_address, sentinel_master_name,
		sentinel_username, sentinel_password_ciphertext, sentinel_tls_enabled, sentinel_tls_config_json,
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

func (s *Store) UpdateConnection(ctx context.Context, connection domain.Connection) error {
	result, err := s.db.ExecContext(ctx, `UPDATE connections SET
		name = ?, topology = ?, address = ?, username = ?, password_ciphertext = ?,
		tls_enabled = ?, tls_config_json = ?, sentinel_address = ?, sentinel_master_name = ?,
		sentinel_username = ?, sentinel_password_ciphertext = ?, sentinel_tls_enabled = ?, sentinel_tls_config_json = ?,
		updated_at = ?, last_tested_at = ?, last_test_result_json = ?
		WHERE id = ?`,
		connection.Name,
		connection.Topology,
		connection.Address,
		connection.Username,
		connection.PasswordCiphertext,
		boolToInt(connection.TLSEnabled),
		emptyJSON(connection.TLSConfigJSON),
		connection.SentinelAddress,
		connection.SentinelMasterName,
		connection.SentinelUsername,
		connection.SentinelPasswordCiphertext,
		boolToInt(connection.SentinelTLSEnabled),
		emptyJSON(connection.SentinelTLSConfigJSON),
		formatTime(connection.UpdatedAt),
		formatOptionalTime(connection.LastTestedAt),
		connection.LastTestResultJSON,
		connection.ID,
	)
	if err != nil {
		return fmt.Errorf("update connection: %w", mapWriteError(err))
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated connection count: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM connections WHERE id = ?", id)
	if err != nil {
		mapped := mapWriteError(err)
		if errors.Is(mapped, ErrConflict) {
			return ErrInUse
		}
		return fmt.Errorf("delete connection: %w", mapped)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted connection count: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateConnectionTestResult(ctx context.Context, id string, testedAt time.Time, resultJSON string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE connections
		SET last_tested_at = ?, last_test_result_json = ?, updated_at = ?
		WHERE id = ?`, formatTime(testedAt), resultJSON, formatTime(testedAt), id)
	if err != nil {
		return fmt.Errorf("update connection test result: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read tested connection count: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateTask(ctx context.Context, task domain.Task) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tasks (
		id, name, description, mode, source_connection_id, target_connection_id,
		reader_options_json, filter_options_json, advanced_options_json,
		state, config_revision, created_at, updated_at, last_prechecked_at, last_precheck_result_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID,
		task.Name,
		task.Description,
		task.Mode,
		optionalString(task.SourceConnectionID),
		optionalString(task.TargetConnectionID),
		emptyJSON(task.ReaderOptionsJSON),
		emptyJSON(task.FilterOptionsJSON),
		emptyJSON(task.AdvancedOptionsJSON),
		task.State,
		task.ConfigRevision,
		formatTime(task.CreatedAt),
		formatTime(task.UpdatedAt),
		formatOptionalTime(task.LastPrecheckedAt),
		task.LastPrecheckResultJSON,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", mapWriteError(err))
	}
	return nil
}

func (s *Store) GetTask(ctx context.Context, id string) (domain.Task, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, name, description, mode, source_connection_id, target_connection_id,
		reader_options_json, filter_options_json, advanced_options_json,
		state, config_revision, created_at, updated_at, last_prechecked_at, last_precheck_result_json
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (s *Store) ListTasks(ctx context.Context) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, name, description, mode, source_connection_id, target_connection_id,
		reader_options_json, filter_options_json, advanced_options_json,
		state, config_revision, created_at, updated_at, last_prechecked_at, last_precheck_result_json
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

func (s *Store) UpdateTask(ctx context.Context, task domain.Task, expectedRevision int64) (int64, error) {
	newRevision := expectedRevision + 1
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET
		name = ?, description = ?, mode = ?, source_connection_id = ?, target_connection_id = ?,
		reader_options_json = ?, filter_options_json = ?, advanced_options_json = ?,
		state = 'DRAFT', config_revision = ?, updated_at = ?,
		last_prechecked_at = NULL, last_precheck_result_json = ''
		WHERE id = ? AND config_revision = ? AND state <> 'ARCHIVED'`,
		task.Name,
		task.Description,
		task.Mode,
		optionalString(task.SourceConnectionID),
		optionalString(task.TargetConnectionID),
		emptyJSON(task.ReaderOptionsJSON),
		emptyJSON(task.FilterOptionsJSON),
		emptyJSON(task.AdvancedOptionsJSON),
		newRevision,
		formatTime(task.UpdatedAt),
		task.ID,
		expectedRevision,
	)
	if err != nil {
		return 0, fmt.Errorf("update task: %w", mapWriteError(err))
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read updated task count: %w", err)
	}
	if rows == 0 {
		return 0, s.taskWriteMiss(ctx, task.ID, expectedRevision)
	}
	return newRevision, nil
}

func (s *Store) MarkTaskReady(ctx context.Context, id string, revision int64, checkedAt time.Time, resultJSON string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET
		state = 'READY', updated_at = ?, last_prechecked_at = ?, last_precheck_result_json = ?
		WHERE id = ? AND config_revision = ? AND state <> 'ARCHIVED'`,
		formatTime(checkedAt), formatTime(checkedAt), resultJSON, id, revision)
	if err != nil {
		return fmt.Errorf("mark task ready: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read ready task count: %w", err)
	}
	if rows == 0 {
		return s.taskWriteMiss(ctx, id, revision)
	}
	return nil
}

func (s *Store) SaveTaskPrecheckResult(ctx context.Context, id string, revision int64, checkedAt time.Time, resultJSON string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET
		state = 'DRAFT', updated_at = ?, last_prechecked_at = ?, last_precheck_result_json = ?
		WHERE id = ? AND config_revision = ? AND state <> 'ARCHIVED'`,
		formatTime(checkedAt), formatTime(checkedAt), resultJSON, id, revision)
	if err != nil {
		return fmt.Errorf("save task precheck result: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read prechecked task count: %w", err)
	}
	if rows == 0 {
		return s.taskWriteMiss(ctx, id, revision)
	}
	return nil
}

func (s *Store) ArchiveTask(ctx context.Context, id string, updatedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET state = 'ARCHIVED', updated_at = ?
		WHERE id = ? AND state <> 'ARCHIVED' AND NOT EXISTS (
			SELECT 1 FROM runs
			WHERE task_id = tasks.id AND state IN ('STARTING', 'RUNNING', 'STOPPING', 'UNKNOWN')
		)`, formatTime(updatedAt), id)
	if err != nil {
		return fmt.Errorf("archive task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read archived task count: %w", err)
	}
	if rows > 0 {
		return nil
	}
	var state domain.TaskState
	if err := s.db.QueryRowContext(ctx, "SELECT state FROM tasks WHERE id = ?", id).Scan(&state); err != nil {
		return mapNotFound(err)
	}
	if state == domain.TaskStateArchived {
		return nil
	}
	return ErrInUse
}

func (s *Store) CreateRun(ctx context.Context, run domain.Run) error {
	return insertRun(ctx, s.db, run)
}

func (s *Store) CreateRunIfNoActive(ctx context.Context, run domain.Run) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin run creation: %w", err)
	}
	defer tx.Rollback()
	var state domain.TaskState
	var revision int64
	if err := tx.QueryRowContext(ctx, "SELECT state, config_revision FROM tasks WHERE id = ?", run.TaskID).Scan(&state, &revision); err != nil {
		return mapNotFound(err)
	}
	if state != domain.TaskStateReady || revision != run.ConfigRevision {
		return ErrTaskNotReady
	}
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM runs WHERE task_id = ? AND state IN ('STARTING', 'RUNNING', 'STOPPING', 'UNKNOWN')
	)`, run.TaskID).Scan(&active); err != nil {
		return fmt.Errorf("check active task run: %w", err)
	}
	if active == 1 {
		return ErrActiveRun
	}
	if err := insertRun(ctx, tx, run); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit run creation: %w", err)
	}
	return nil
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertRun(ctx context.Context, executor sqlExecer, run domain.Run) error {
	updatedAt := run.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = run.StartedAt
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO runs (
		id, task_id, config_revision, config_snapshot_json, state,
		pid, process_started_at, status_port, runtime_dir, started_at,
		finished_at, exit_code, exit_reason, last_heartbeat_at, stop_requested_by_user,
		status_json, status_healthy, worker_path, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		run.StatusJSON,
		boolToInt(run.StatusHealthy),
		run.WorkerPath,
		formatTime(updatedAt),
	)
	if err != nil {
		return fmt.Errorf("create run: %w", mapWriteError(err))
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, id string) (domain.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, task_id, config_revision, config_snapshot_json, state,
		pid, process_started_at, status_port, runtime_dir, started_at,
		finished_at, exit_code, exit_reason, last_heartbeat_at, stop_requested_by_user,
		status_json, status_healthy, worker_path, updated_at
		FROM runs WHERE id = ?`, id)
	return scanRun(row)
}

func (s *Store) ListRunsByTask(ctx context.Context, taskID string) ([]domain.Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, task_id, config_revision, config_snapshot_json, state,
		pid, process_started_at, status_port, runtime_dir, started_at,
		finished_at, exit_code, exit_reason, last_heartbeat_at, stop_requested_by_user,
		status_json, status_healthy, worker_path, updated_at
		FROM runs WHERE task_id = ? ORDER BY started_at DESC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task runs: %w", err)
	}
	defer rows.Close()
	runs := make([]domain.Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task runs: %w", err)
	}
	return runs, nil
}

func (s *Store) UpdateRunStarted(ctx context.Context, id string, pid int, processStartedAt time.Time, statusPort int, workerPath string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET
		pid = ?, process_started_at = ?, status_port = ?, worker_path = ?, updated_at = ?
		WHERE id = ? AND state = 'STARTING'`, pid, formatTime(processStartedAt), statusPort, workerPath, formatTime(processStartedAt), id)
	if err != nil {
		return fmt.Errorf("update started run: %w", err)
	}
	return requireChanged(result, "started run")
}

func (s *Store) MarkRunRunning(ctx context.Context, id string, statusJSON string, heartbeatAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET
		state = 'RUNNING', status_json = ?, status_healthy = 1,
		last_heartbeat_at = ?, updated_at = ?
		WHERE id = ? AND state = 'STARTING'`, statusJSON, formatTime(heartbeatAt), formatTime(heartbeatAt), id)
	if err != nil {
		return fmt.Errorf("mark run running: %w", err)
	}
	return requireChanged(result, "running run")
}

func (s *Store) UpdateRunStatus(ctx context.Context, id string, statusJSON string, heartbeatAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET
		status_json = ?, status_healthy = 1, last_heartbeat_at = ?, updated_at = ?
		WHERE id = ? AND state IN ('RUNNING', 'STOPPING')`, statusJSON, formatTime(heartbeatAt), formatTime(heartbeatAt), id)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	return requireChanged(result, "run status")
}

func (s *Store) MarkRunStatusUnhealthy(ctx context.Context, id string, updatedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET status_healthy = 0, updated_at = ?
		WHERE id = ? AND state IN ('STARTING', 'RUNNING', 'STOPPING')`, formatTime(updatedAt), id)
	if err != nil {
		return fmt.Errorf("mark run status unhealthy: %w", err)
	}
	return requireChanged(result, "unhealthy run status")
}

func (s *Store) RequestRunStop(ctx context.Context, id string, requestedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET
		state = 'STOPPING', stop_requested_by_user = 1, updated_at = ?
		WHERE id = ? AND state IN ('STARTING', 'RUNNING')`, formatTime(requestedAt), id)
	if err != nil {
		return fmt.Errorf("request run stop: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read stopped run count: %w", err)
	}
	if rows > 0 {
		return nil
	}
	var state domain.RunState
	if err := s.db.QueryRowContext(ctx, "SELECT state FROM runs WHERE id = ?", id).Scan(&state); err != nil {
		return mapNotFound(err)
	}
	if state == domain.RunStateStopping {
		return nil
	}
	return ErrConflict
}

func (s *Store) FinalizeRun(ctx context.Context, id string, state domain.RunState, exitCode *int, reason string, finishedAt time.Time) error {
	if state != domain.RunStateStopped && state != domain.RunStateSucceeded && state != domain.RunStateFailed {
		return fmt.Errorf("invalid final run state %s", state)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET
		state = ?, finished_at = ?, exit_code = ?, exit_reason = ?, status_healthy = 0, updated_at = ?
		WHERE id = ? AND state IN ('STARTING', 'RUNNING', 'STOPPING', 'UNKNOWN')`,
		state, formatTime(finishedAt), optionalInt(exitCode), reason, formatTime(finishedAt), id)
	if err != nil {
		return fmt.Errorf("finalize run: %w", err)
	}
	return requireChanged(result, "finalized run")
}

func (s *Store) MarkActiveRunsUnknown(ctx context.Context, reason string, updatedAt time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET
		state = 'UNKNOWN', status_healthy = 0, exit_reason = ?, updated_at = ?
		WHERE state IN ('STARTING', 'RUNNING', 'STOPPING')`, reason, formatTime(updatedAt))
	if err != nil {
		return 0, fmt.Errorf("mark active runs unknown: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read unknown run count: %w", err)
	}
	return count, nil
}

func requireChanged(result sql.Result, operation string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s count: %w", operation, err)
	}
	if rows == 0 {
		return ErrConflict
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConnection(row scanner) (domain.Connection, error) {
	var connection domain.Connection
	var tlsEnabled, sentinelTLSEnabled int
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
		&connection.SentinelAddress,
		&connection.SentinelMasterName,
		&connection.SentinelUsername,
		&connection.SentinelPasswordCiphertext,
		&sentinelTLSEnabled,
		&connection.SentinelTLSConfigJSON,
		&createdAt,
		&updatedAt,
		&lastTestedAt,
		&connection.LastTestResultJSON,
	)
	if err != nil {
		return domain.Connection{}, mapNotFound(err)
	}
	connection.TLSEnabled = tlsEnabled == 1
	connection.SentinelTLSEnabled = sentinelTLSEnabled == 1
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
	var lastPrecheckedAt sql.NullString
	var sourceConnectionID, targetConnectionID sql.NullString
	err := row.Scan(
		&task.ID,
		&task.Name,
		&task.Description,
		&task.Mode,
		&sourceConnectionID,
		&targetConnectionID,
		&task.ReaderOptionsJSON,
		&task.FilterOptionsJSON,
		&task.AdvancedOptionsJSON,
		&task.State,
		&task.ConfigRevision,
		&createdAt,
		&updatedAt,
		&lastPrecheckedAt,
		&task.LastPrecheckResultJSON,
	)
	if err != nil {
		return domain.Task{}, mapNotFound(err)
	}
	task.SourceConnectionID = sourceConnectionID.String
	task.TargetConnectionID = targetConnectionID.String
	if task.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Task{}, err
	}
	if task.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Task{}, err
	}
	if task.LastPrecheckedAt, err = parseOptionalTime(lastPrecheckedAt); err != nil {
		return domain.Task{}, err
	}
	return task, nil
}

func (s *Store) taskWriteMiss(ctx context.Context, id string, expectedRevision int64) error {
	var revision int64
	if err := s.db.QueryRowContext(ctx, "SELECT config_revision FROM tasks WHERE id = ?", id).Scan(&revision); err != nil {
		return mapNotFound(err)
	}
	if revision != expectedRevision {
		return ErrRevisionConflict
	}
	return ErrConflict
}

func scanRun(row scanner) (domain.Run, error) {
	var run domain.Run
	var pid, statusPort, exitCode sql.NullInt64
	var processStartedAt, finishedAt, heartbeatAt, updatedAt sql.NullString
	var startedAt string
	var stopRequested, statusHealthy int
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
		&run.StatusJSON,
		&statusHealthy,
		&run.WorkerPath,
		&updatedAt,
	)
	if err != nil {
		return domain.Run{}, mapNotFound(err)
	}
	run.PID = nullIntToPointer(pid)
	run.StatusPort = nullIntToPointer(statusPort)
	run.ExitCode = nullIntToPointer(exitCode)
	run.StopRequestedByUser = stopRequested == 1
	run.StatusHealthy = statusHealthy == 1
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
	if updatedAt.Valid {
		if run.UpdatedAt, err = parseTime(updatedAt.String); err != nil {
			return domain.Run{}, err
		}
	} else {
		run.UpdatedAt = run.StartedAt
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

func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
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

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint failed") || strings.Contains(message, "foreign key constraint failed") {
		return fmt.Errorf("%w: %s", ErrConflict, err.Error())
	}
	return err
}
