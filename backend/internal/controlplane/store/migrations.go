package store

type migration struct {
	version            int
	statements         []string
	disableForeignKeys bool
}

var migrations = []migration{
	{
		version: 1,
		statements: []string{
			`CREATE TABLE connections (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL COLLATE NOCASE UNIQUE,
				topology TEXT NOT NULL CHECK (topology IN ('standalone', 'sentinel', 'cluster')),
				address TEXT NOT NULL,
				username TEXT NOT NULL DEFAULT '',
				password_ciphertext TEXT NOT NULL DEFAULT '',
				tls_enabled INTEGER NOT NULL DEFAULT 0 CHECK (tls_enabled IN (0, 1)),
				tls_config_json TEXT NOT NULL DEFAULT '{}',
				sentinel_master_name TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				last_tested_at TEXT,
				last_test_result_json TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE TABLE tasks (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL COLLATE NOCASE UNIQUE,
				description TEXT NOT NULL DEFAULT '',
				mode TEXT NOT NULL CHECK (mode IN ('sync', 'scan')),
				source_connection_id TEXT NOT NULL,
				target_connection_id TEXT NOT NULL,
				reader_options_json TEXT NOT NULL DEFAULT '{}',
				filter_options_json TEXT NOT NULL DEFAULT '{}',
				advanced_options_json TEXT NOT NULL DEFAULT '{}',
				state TEXT NOT NULL CHECK (state IN ('DRAFT', 'READY', 'ARCHIVED')),
				config_revision INTEGER NOT NULL DEFAULT 1 CHECK (config_revision > 0),
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY (source_connection_id) REFERENCES connections(id) ON DELETE RESTRICT,
				FOREIGN KEY (target_connection_id) REFERENCES connections(id) ON DELETE RESTRICT
			)`,
			`CREATE TABLE runs (
				id TEXT PRIMARY KEY,
				task_id TEXT NOT NULL,
				config_revision INTEGER NOT NULL CHECK (config_revision > 0),
				config_snapshot_json TEXT NOT NULL DEFAULT '{}',
				state TEXT NOT NULL CHECK (state IN ('STARTING', 'RUNNING', 'STOPPING', 'STOPPED', 'SUCCEEDED', 'FAILED', 'UNKNOWN')),
				pid INTEGER,
				process_started_at TEXT,
				status_port INTEGER CHECK (status_port IS NULL OR (status_port > 0 AND status_port < 65536)),
				runtime_dir TEXT NOT NULL,
				started_at TEXT NOT NULL,
				finished_at TEXT,
				exit_code INTEGER,
				exit_reason TEXT NOT NULL DEFAULT '',
				last_heartbeat_at TEXT,
				stop_requested_by_user INTEGER NOT NULL DEFAULT 0 CHECK (stop_requested_by_user IN (0, 1)),
				FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE RESTRICT
			)`,
			`CREATE INDEX idx_tasks_state_updated_at ON tasks(state, updated_at DESC)`,
			`CREATE INDEX idx_runs_task_started_at ON runs(task_id, started_at DESC)`,
			`CREATE INDEX idx_runs_state ON runs(state)`,
		},
	},
	{
		version: 2,
		statements: []string{
			`ALTER TABLE connections ADD COLUMN sentinel_address TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE connections ADD COLUMN sentinel_username TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE connections ADD COLUMN sentinel_password_ciphertext TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE connections ADD COLUMN sentinel_tls_enabled INTEGER NOT NULL DEFAULT 0 CHECK (sentinel_tls_enabled IN (0, 1))`,
			`ALTER TABLE connections ADD COLUMN sentinel_tls_config_json TEXT NOT NULL DEFAULT '{}'`,
		},
	},
	{
		version:            3,
		disableForeignKeys: true,
		statements: []string{
			`CREATE TABLE new_tasks (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL COLLATE NOCASE UNIQUE,
				description TEXT NOT NULL DEFAULT '',
				mode TEXT NOT NULL CHECK (mode IN ('sync', 'scan')),
				source_connection_id TEXT,
				target_connection_id TEXT,
				reader_options_json TEXT NOT NULL DEFAULT '{}',
				filter_options_json TEXT NOT NULL DEFAULT '{}',
				advanced_options_json TEXT NOT NULL DEFAULT '{}',
				state TEXT NOT NULL CHECK (state IN ('DRAFT', 'READY', 'ARCHIVED')),
				config_revision INTEGER NOT NULL DEFAULT 1 CHECK (config_revision > 0),
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				last_prechecked_at TEXT,
				last_precheck_result_json TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (source_connection_id) REFERENCES connections(id) ON DELETE RESTRICT,
				FOREIGN KEY (target_connection_id) REFERENCES connections(id) ON DELETE RESTRICT
			)`,
			`INSERT INTO new_tasks (
				id, name, description, mode, source_connection_id, target_connection_id,
				reader_options_json, filter_options_json, advanced_options_json,
				state, config_revision, created_at, updated_at
			) SELECT
				id, name, description, mode, source_connection_id, target_connection_id,
				reader_options_json, filter_options_json, advanced_options_json,
				state, config_revision, created_at, updated_at
			FROM tasks`,
			`DROP TABLE tasks`,
			`ALTER TABLE new_tasks RENAME TO tasks`,
			`CREATE INDEX idx_tasks_state_updated_at ON tasks(state, updated_at DESC)`,
		},
	},
}
