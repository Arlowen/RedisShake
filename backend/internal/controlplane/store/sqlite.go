package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound         = errors.New("record not found")
	ErrConflict         = errors.New("record conflicts with existing data")
	ErrInUse            = errors.New("record is still in use")
	ErrRevisionConflict = errors.New("record revision has changed")
	ErrTaskNotReady     = errors.New("task is not ready")
	ErrActiveRun        = errors.New("task already has an active run")
	ErrConcurrencyLimit = errors.New("global run concurrency limit reached")
)

type Store struct {
	db   *sql.DB
	path string
}

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	// A single writer connection is sufficient for the first single-node
	// control plane and makes PRAGMA settings deterministic for every query.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db, path: path}
	if err := store.configure(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("protect database file: %w", err)
	}
	return store, nil
}

func (s *Store) configure(ctx context.Context) error {
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure sqlite with %q: %w", statement, err)
		}
	}
	return nil
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	var current int
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for _, item := range migrations {
		if item.version <= current {
			continue
		}
		if item.disableForeignKeys {
			if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
				return fmt.Errorf("disable foreign keys for migration %d: %w", item.version, err)
			}
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			if item.disableForeignKeys {
				_, _ = s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
			}
			return fmt.Errorf("begin migration %d: %w", item.version, err)
		}
		for _, statement := range item.statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				tx.Rollback()
				if item.disableForeignKeys {
					_, _ = s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
				}
				return fmt.Errorf("apply migration %d: %w", item.version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (?)", item.version); err != nil {
			tx.Rollback()
			if item.disableForeignKeys {
				_, _ = s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
			}
			return fmt.Errorf("record migration %d: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			if item.disableForeignKeys {
				_, _ = s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
			}
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
		if item.disableForeignKeys {
			if err := s.verifyForeignKeys(ctx); err != nil {
				_, _ = s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
				return fmt.Errorf("verify foreign keys after migration %d: %w", item.version, err)
			}
			if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
				return fmt.Errorf("enable foreign keys after migration %d: %w", item.version, err)
			}
		}
		current = item.version
	}
	return nil
}

func (s *Store) verifyForeignKeys(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowID sql.NullInt64
		var parent string
		var foreignKeyID int
		if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			return err
		}
		return fmt.Errorf("foreign key violation in table %s referencing %s", table, parent)
	}
	return rows.Err()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("database is not open")
	}
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite database: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) HasEncryptedSecrets(ctx context.Context) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM connections
		WHERE password_ciphertext <> ''
		   OR sentinel_password_ciphertext <> ''
		   OR tls_config_json LIKE '%v1:%'
		   OR sentinel_tls_config_json LIKE '%v1:%'
		LIMIT 1
	)`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check encrypted secrets: %w", err)
	}
	return exists == 1, nil
}
