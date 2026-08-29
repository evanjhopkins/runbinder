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
	if summaries[0].Definition != app.DefinitionOK {
		t.Fatalf("definition state = %q, want %q", summaries[0].Definition, app.DefinitionOK)
	}

	changedDefinition := "namespace: example.backup\ncommand: echo changed\ncron: 0 * * * *\nworking_dir: project\n"
	if err := os.WriteFile(definitionPath, []byte(changedDefinition), 0o644); err != nil {
		t.Fatal(err)
	}
	assertDefinitionState(t, application, ctx, app.DefinitionChanged)

	invalidDefinition := "namespace: example.backup\ncommand: echo changed\ncron: not-a-cron\n"
	if err := os.WriteFile(definitionPath, []byte(invalidDefinition), 0o644); err != nil {
		t.Fatal(err)
	}
	assertDefinitionState(t, application, ctx, app.DefinitionInvalid)

	if err := os.Remove(definitionPath); err != nil {
		t.Fatal(err)
	}
	assertDefinitionState(t, application, ctx, app.DefinitionMissing)

	removed, err := application.Tasks.Remove(ctx, "example.backup")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Namespace != "example.backup" {
		t.Fatalf("removed namespace = %q", removed.Namespace)
	}
}

func assertDefinitionState(t *testing.T, application *app.Application, ctx context.Context, want app.DefinitionState) {
	t.Helper()
	summaries, err := application.Tasks.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].Definition != want {
		t.Fatalf("definition summaries = %#v, want state %q", summaries, want)
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
