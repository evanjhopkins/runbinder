package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/evanjhopkins/RunBinder/internal/app"
	"github.com/evanjhopkins/RunBinder/internal/platform"
)

func TestTaskLifecycleFromDefinition(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workingDir := filepath.Join(root, "project")
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	definitionPath := filepath.Join(root, "task.runbinder.yaml")
	definition := "namespace: example.backup\ncommand: echo backup\ncron: 0 * * * *\nworking_dir: project\n"
	if err := os.WriteFile(definitionPath, []byte(definition), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := testPaths(filepath.Join(root, "state"))
	application := app.New(paths)
	ctx := context.Background()
	task, err := application.Tasks.Add(ctx, definitionPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if task.WorkingDir != workingDir {
		t.Fatalf("working directory = %q, want %q", task.WorkingDir, workingDir)
	}

	task, changed, err := application.Tasks.SetActive(ctx, "example.backup", true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !task.Active {
		t.Fatalf("task was not enabled: changed=%t active=%t", changed, task.Active)
	}

	summaries, err := application.Tasks.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].Task.Namespace != "example.backup" {
		t.Fatalf("unexpected task summaries: %#v", summaries)
	}

	removed, err := application.Tasks.Remove(ctx, definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	if removed.Namespace != "example.backup" {
		t.Fatalf("removed namespace = %q", removed.Namespace)
	}
}

func testPaths(storageDir string) platform.Paths {
	return platform.Paths{
		StorageDir:  storageDir,
		Database:    filepath.Join(storageDir, "runbinder.db"),
		InternalLog: filepath.Join(storageDir, "runbinder.log"),
		ServiceLock: filepath.Join(storageDir, "service.lock"),
		ServicePID:  filepath.Join(storageDir, "service.pid"),
	}
}
