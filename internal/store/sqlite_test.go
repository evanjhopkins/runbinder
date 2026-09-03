package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	_ "modernc.org/sqlite"
)

func TestSQLiteTaskLifecycle(t *testing.T) {
	repository, err := OpenSQLite(filepath.Join(t.TempDir(), "runbinder.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	ctx := context.Background()
	task := domain.Task{
		Namespace: "example", Definition: "definition", Hash: "one",
		WorkingDir: t.TempDir(), SourcePath: "/tmp/example.runbinder.yaml",
	}
	created, err := repository.AddTask(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatal("expected generated task ID")
	}
	if _, err := repository.AddTask(ctx, task); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate add error = %v", err)
	}
	if err := repository.SetTaskActive(ctx, task.Namespace, true); err != nil {
		t.Fatal(err)
	}
	enabled, err := repository.ListEnabledTasks(ctx)
	if err != nil || len(enabled) != 1 {
		t.Fatalf("enabled tasks = %v, err = %v", enabled, err)
	}

	task.Definition = "changed"
	task.Hash = "two"
	updated, err := repository.UpdateTask(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Active {
		t.Fatal("update did not preserve active state")
	}
	if _, err := repository.UpdateTask(ctx, task); !errors.Is(err, ErrNoChanges) {
		t.Fatalf("unchanged update error = %v", err)
	}

	now := time.Now().Truncate(time.Microsecond)
	if err := repository.RecordRun(ctx, domain.Run{
		Namespace: task.Namespace, ScheduledAt: now.Add(-time.Second), StartedAt: now,
		FinishedAt: now.Add(time.Second), Success: true,
	}); err != nil {
		t.Fatal(err)
	}
	run, err := repository.LastRun(ctx, task.Namespace)
	if err != nil || run == nil || !run.Success || !run.StartedAt.Equal(now) {
		t.Fatalf("last run = %#v, err = %v", run, err)
	}
	if err := repository.UpdateHeartbeat(ctx, "service", now); err != nil {
		t.Fatal(err)
	}
	heartbeat, err := repository.Heartbeat(ctx, "service")
	if err != nil || heartbeat == nil || !heartbeat.Running || !heartbeat.Last.Equal(now) || !heartbeat.StartedAt.Equal(now) {
		t.Fatalf("heartbeat = %v, err = %v", heartbeat, err)
	}
	if err := repository.UpdateHeartbeat(ctx, "service", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	heartbeat, err = repository.Heartbeat(ctx, "service")
	if err != nil || heartbeat == nil || !heartbeat.StartedAt.Equal(now) {
		t.Fatalf("heartbeat start changed during update: %v, err = %v", heartbeat, err)
	}
	if err := repository.StopHeartbeat(ctx, "service", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	heartbeat, err = repository.Heartbeat(ctx, "service")
	if err != nil || heartbeat == nil || heartbeat.Running {
		t.Fatalf("stopped heartbeat = %v, err = %v", heartbeat, err)
	}
	restartedAt := now.Add(2 * time.Minute)
	if err := repository.UpdateHeartbeat(ctx, "service", restartedAt); err != nil {
		t.Fatal(err)
	}
	heartbeat, err = repository.Heartbeat(ctx, "service")
	if err != nil || heartbeat == nil || !heartbeat.StartedAt.Equal(restartedAt) {
		t.Fatalf("heartbeat start was not reset: %v, err = %v", heartbeat, err)
	}
	if err := repository.RemoveTask(ctx, task.Namespace); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Task(ctx, task.Namespace); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed task error = %v", err)
	}
}

func TestSQLiteRecordsSchemaVersion(t *testing.T) {
	repository, err := OpenSQLite(filepath.Join(t.TempDir(), "runbinder.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	var version int
	if err := repository.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("schema version = %d, want 2", version)
	}
	columns, err := tableColumns(repository.db, "tasks")
	if err != nil {
		t.Fatal(err)
	}
	if !columns["hash"] {
		t.Fatal("tasks.hash is missing")
	}
	if columns["md5"] {
		t.Fatal("tasks.md5 should not exist")
	}
}

func TestSQLiteRejectsUnversionedLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runbinder.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE Tasks (id INTEGER PRIMARY KEY, namespace TEXT UNIQUE NOT NULL, active BOOLEAN NOT NULL, definition TEXT NOT NULL, md5 TEXT NOT NULL, working_dir TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenSQLite(path); err == nil {
		t.Fatal("expected unversioned schema to be rejected")
	}
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}
