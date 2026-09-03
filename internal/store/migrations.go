package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type migration struct {
	version    int
	statements []string
}

var migrations = []migration{
	{
		version: 1,
		statements: []string{
			`CREATE TABLE tasks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				namespace TEXT NOT NULL UNIQUE,
				active INTEGER NOT NULL DEFAULT 0 CHECK (active IN (0, 1)),
				definition TEXT NOT NULL,
				hash TEXT NOT NULL,
				working_dir TEXT NOT NULL,
				source_path TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE TABLE service_heartbeats (
				name TEXT PRIMARY KEY,
				last_at TEXT NOT NULL,
				running INTEGER NOT NULL DEFAULT 0 CHECK (running IN (0, 1))
			)`,
			`CREATE TABLE runs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				namespace TEXT NOT NULL,
				scheduled_at TEXT NOT NULL,
				started_at TEXT NOT NULL,
				finished_at TEXT NOT NULL,
				success INTEGER NOT NULL CHECK (success IN (0, 1)),
				error TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX idx_runs_namespace_started_at ON runs(namespace, started_at DESC)`,
		},
	},
	{
		version: 2,
		statements: []string{
			`ALTER TABLE service_heartbeats ADD COLUMN started_at TEXT NOT NULL DEFAULT ''`,
			`UPDATE service_heartbeats SET started_at = last_at WHERE started_at = ''`,
		},
	},
}

func (s *SQLite) migrate(ctx context.Context) error {
	for _, statement := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure database: %w", err)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	applied, err := appliedMigrations(ctx, tx)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		legacy, err := hasLegacySchema(ctx, tx)
		if err != nil {
			return err
		}
		if legacy {
			return errors.New("database predates schema migrations; run `runbinder nuke --yes` to reset local development state")
		}
	}
	for _, migration := range migrations {
		if applied[migration.version] {
			continue
		}
		for _, statement := range migration.statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply migration %d: %w", migration.version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, datetime('now'))`, migration.version); err != nil {
			return fmt.Errorf("record migration %d: %w", migration.version, err)
		}
	}
	if err := rejectUnknownMigrations(applied); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migration: %w", err)
	}
	return nil
}

func appliedMigrations(ctx context.Context, tx *sql.Tx) (map[int]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("read schema migration: %w", err)
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func hasLegacySchema(ctx context.Context, tx *sql.Tx) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND lower(name) IN ('tasks', 'run', 'heartbeat')`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("inspect legacy schema: %w", err)
	}
	return count > 0, nil
}

func rejectUnknownMigrations(applied map[int]bool) error {
	known := make(map[int]bool, len(migrations))
	for _, migration := range migrations {
		known[migration.version] = true
	}
	for version := range applied {
		if !known[version] {
			return fmt.Errorf("database schema version %d is newer than this RunBinder binary", version)
		}
	}
	return nil
}
