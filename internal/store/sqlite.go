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

func (s *SQLite) AddTask(ctx context.Context, task domain.Task) (domain.Task, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO tasks(namespace, active, definition, hash, working_dir, source_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Namespace, task.Active, task.Definition, task.Hash, task.WorkingDir,
		task.SourcePath, formatTime(now), formatTime(now),
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
		UPDATE tasks SET definition = ?, hash = ?, working_dir = ?, source_path = ?, updated_at = ?
		WHERE namespace = ?`, task.Definition, task.Hash, task.WorkingDir,
		task.SourcePath, formatTime(now), task.Namespace)
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
		SELECT id, namespace, active, definition, hash, working_dir,
		       source_path, created_at, updated_at
		FROM tasks WHERE namespace = ?`, namespace)
	return scanTask(row)
}

func (s *SQLite) ListTasks(ctx context.Context) ([]domain.Task, error) {
	return s.listTasks(ctx, false)
}

func (s *SQLite) ListEnabledTasks(ctx context.Context) ([]domain.Task, error) {
	return s.listTasks(ctx, true)
}

func (s *SQLite) listTasks(ctx context.Context, enabledOnly bool) ([]domain.Task, error) {
	query := `SELECT id, namespace, active, definition, hash, working_dir,
	                 source_path, created_at, updated_at
	          FROM tasks`
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
	result, err := s.db.ExecContext(ctx, `UPDATE tasks SET active = ?, updated_at = ? WHERE namespace = ?`, active, formatTime(time.Now()), namespace)
	if err != nil {
		return fmt.Errorf("change task state: %w", err)
	}
	return requireAffected(result)
}

func (s *SQLite) RemoveTask(ctx context.Context, namespace string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE namespace = ?`, namespace)
	if err != nil {
		return fmt.Errorf("remove task: %w", err)
	}
	return requireAffected(result)
}

func (s *SQLite) LastRun(ctx context.Context, namespace string) (*domain.Run, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, namespace, started_at, success, scheduled_at, finished_at, error
		FROM runs WHERE namespace = ? ORDER BY started_at DESC LIMIT 1`, namespace)
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
		INSERT INTO runs(namespace, scheduled_at, started_at, finished_at, success, error)
		VALUES (?, ?, ?, ?, ?, ?)`, run.Namespace, formatTime(run.ScheduledAt),
		formatTime(run.StartedAt), formatTime(run.FinishedAt), run.Success, run.Error)
	if err != nil {
		return fmt.Errorf("record run: %w", err)
	}
	return nil
}

func (s *SQLite) UpdateHeartbeat(ctx context.Context, name string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO service_heartbeats(name, last_at, running) VALUES (?, ?, 1)
		ON CONFLICT(name) DO UPDATE SET last_at = excluded.last_at, running = 1`, name, formatTime(at))
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}
	return nil
}

func (s *SQLite) StopHeartbeat(ctx context.Context, name string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO service_heartbeats(name, last_at, running) VALUES (?, ?, 0)
		ON CONFLICT(name) DO UPDATE SET last_at = excluded.last_at, running = 0`, name, formatTime(at))
	if err != nil {
		return fmt.Errorf("stop heartbeat: %w", err)
	}
	return nil
}

func (s *SQLite) Heartbeat(ctx context.Context, name string) (*domain.Heartbeat, error) {
	var raw string
	var running bool
	err := s.db.QueryRowContext(ctx, `SELECT last_at, running FROM service_heartbeats WHERE name = ?`, name).Scan(&raw, &running)
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
