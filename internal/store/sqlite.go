package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLite struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &SQLite{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS Tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			namespace TEXT NOT NULL UNIQUE,
			active INTEGER NOT NULL DEFAULT 0,
			definition TEXT NOT NULL,
			md5 TEXT NOT NULL,
			working_dir TEXT NOT NULL,
			source_path TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS HeartBeat (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			last TEXT NOT NULL,
			running INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS Run (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			namespace TEXT NOT NULL,
			execution_time TEXT NOT NULL,
			status INTEGER NOT NULL,
			scheduled_at TEXT,
			finished_at TEXT,
			error TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_run_namespace_time ON Run(namespace, execution_time DESC)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
	}

	// Pony-created databases predate these fields. Add them in place so existing
	// registrations and history remain usable after installing the Go binary.
	columns := map[string]string{
		"source_path": "TEXT",
		"created_at":  "TEXT",
		"updated_at":  "TEXT",
	}
	if err := s.ensureColumns(ctx, "Tasks", columns); err != nil {
		return err
	}
	if err := s.ensureColumns(ctx, "Run", map[string]string{
		"scheduled_at": "TEXT",
		"finished_at":  "TEXT",
		"error":        "TEXT",
	}); err != nil {
		return err
	}
	return s.ensureColumns(ctx, "HeartBeat", map[string]string{
		"running": "INTEGER NOT NULL DEFAULT 0",
	})
}

func (s *SQLite) ensureColumns(ctx context.Context, table string, wanted map[string]string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("inspect %s schema: %w", table, err)
	}
	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		existing[strings.ToLower(name)] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for name, kind := range wanted {
		if existing[name] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+name+` `+kind); err != nil {
			return fmt.Errorf("add %s.%s: %w", table, name, err)
		}
	}
	return nil
}

func (s *SQLite) AddTask(ctx context.Context, task domain.Task) (domain.Task, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO Tasks(namespace, active, definition, md5, working_dir, source_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Namespace, task.Active, task.Definition, task.Hash, task.WorkingDir,
		nullIfEmpty(task.SourcePath), formatTime(now), formatTime(now),
	)
	if isUniqueError(err) {
		return domain.Task{}, ErrAlreadyExists
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("add task: %w", err)
	}
	task.ID, _ = result.LastInsertId()
	task.CreatedAt = now
	task.UpdatedAt = now
	return task, nil
}

func (s *SQLite) UpdateTask(ctx context.Context, task domain.Task) (domain.Task, error) {
	current, err := s.Task(ctx, task.Namespace)
	if err != nil {
		return domain.Task{}, err
	}
	if current.Hash == task.Hash && current.WorkingDir == task.WorkingDir && current.SourcePath == task.SourcePath {
		return domain.Task{}, ErrNoChanges
	}
	now := time.Now()
	_, err = s.db.ExecContext(ctx, `
		UPDATE Tasks SET definition = ?, md5 = ?, working_dir = ?, source_path = ?, updated_at = ?
		WHERE namespace = ?`, task.Definition, task.Hash, task.WorkingDir,
		nullIfEmpty(task.SourcePath), formatTime(now), task.Namespace)
	if err != nil {
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	task.ID = current.ID
	task.Active = current.Active
	task.CreatedAt = current.CreatedAt
	task.UpdatedAt = now
	return task, nil
}

func (s *SQLite) Task(ctx context.Context, namespace string) (domain.Task, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, namespace, active, definition, md5, working_dir,
		       COALESCE(source_path, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM Tasks WHERE namespace = ?`, namespace)
	return scanTask(row)
}

func (s *SQLite) ListTasks(ctx context.Context) ([]domain.Task, error) {
	return s.listTasks(ctx, false)
}

func (s *SQLite) ListEnabledTasks(ctx context.Context) ([]domain.Task, error) {
	return s.listTasks(ctx, true)
}

func (s *SQLite) listTasks(ctx context.Context, enabledOnly bool) ([]domain.Task, error) {
	query := `SELECT id, namespace, active, definition, md5, working_dir,
	                 COALESCE(source_path, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
	          FROM Tasks`
	if enabledOnly {
		query += ` WHERE active = 1`
	}
	query += ` ORDER BY id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *SQLite) SetTaskActive(ctx context.Context, namespace string, active bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE Tasks SET active = ?, updated_at = ? WHERE namespace = ?`, active, formatTime(time.Now()), namespace)
	if err != nil {
		return fmt.Errorf("change task state: %w", err)
	}
	return requireAffected(result)
}

func (s *SQLite) RemoveTask(ctx context.Context, namespace string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM Tasks WHERE namespace = ?`, namespace)
	if err != nil {
		return fmt.Errorf("remove task: %w", err)
	}
	return requireAffected(result)
}

func (s *SQLite) LastRun(ctx context.Context, namespace string) (*domain.Run, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, namespace, execution_time, status,
		       COALESCE(scheduled_at, ''), COALESCE(finished_at, ''), COALESCE(error, '')
		FROM Run WHERE namespace = ? ORDER BY execution_time DESC LIMIT 1`, namespace)
	var run domain.Run
	var started, scheduled, finished string
	if err := row.Scan(&run.ID, &run.Namespace, &started, &run.Success, &scheduled, &finished, &run.Error); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("get last run: %w", err)
	}
	run.StartedAt = parseStoredTime(started)
	run.ScheduledAt = parseStoredTime(scheduled)
	run.FinishedAt = parseStoredTime(finished)
	return &run, nil
}

func (s *SQLite) RecordRun(ctx context.Context, run domain.Run) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO Run(namespace, execution_time, status, scheduled_at, finished_at, error)
		VALUES (?, ?, ?, ?, ?, ?)`, run.Namespace, formatTime(run.StartedAt), run.Success,
		formatTime(run.ScheduledAt), formatTime(run.FinishedAt), nullIfEmpty(run.Error))
	if err != nil {
		return fmt.Errorf("record run: %w", err)
	}
	return nil
}

func (s *SQLite) UpdateHeartbeat(ctx context.Context, name string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO HeartBeat(name, last, running) VALUES (?, ?, 1)
		ON CONFLICT(name) DO UPDATE SET last = excluded.last, running = 1`, name, formatTime(at))
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}
	return nil
}

func (s *SQLite) StopHeartbeat(ctx context.Context, name string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO HeartBeat(name, last, running) VALUES (?, ?, 0)
		ON CONFLICT(name) DO UPDATE SET last = excluded.last, running = 0`, name, formatTime(at))
	if err != nil {
		return fmt.Errorf("stop heartbeat: %w", err)
	}
	return nil
}

func (s *SQLite) Heartbeat(ctx context.Context, name string) (*domain.Heartbeat, error) {
	var raw string
	var running bool
	err := s.db.QueryRowContext(ctx, `SELECT last, running FROM HeartBeat WHERE name = ?`, name).Scan(&raw, &running)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get heartbeat: %w", err)
	}
	return &domain.Heartbeat{Last: parseStoredTime(raw), Running: running}, nil
}

type scanner interface {
	Scan(...any) error
}

func scanTask(row scanner) (domain.Task, error) {
	var task domain.Task
	var created, updated string
	if err := row.Scan(&task.ID, &task.Namespace, &task.Active, &task.Definition, &task.Hash,
		&task.WorkingDir, &task.SourcePath, &created, &updated); errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	} else if err != nil {
		return domain.Task{}, fmt.Errorf("read task: %w", err)
	}
	task.CreatedAt = parseStoredTime(created)
	task.UpdatedAt = parseStoredTime(updated)
	return task, nil
}

func requireAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func parseStoredTime(value string) time.Time {
	formats := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if parsed, err := time.ParseInLocation(format, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
