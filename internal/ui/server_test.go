package ui

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evanjhopkins/RunBinder/internal/app"
	"github.com/evanjhopkins/RunBinder/internal/platform"
)

func TestServerServesDashboardAndTaskData(t *testing.T) {
	root := t.TempDir()
	workingDir := filepath.Join(root, "project")
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	definitionPath := filepath.Join(root, "task.runbinder.yaml")
	definition := "namespace: example.ui\ncommand: echo ui\ncron: 0 * * * *\nworking_dir: project\n"
	if err := os.WriteFile(definitionPath, []byte(definition), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := platform.Paths{
		StorageDir:  filepath.Join(root, "state"),
		Database:    filepath.Join(root, "state", "runbinder.db"),
		InternalLog: filepath.Join(root, "state", "runbinder.log"),
		ServiceLock: filepath.Join(root, "state", "service.lock"),
		ServicePID:  filepath.Join(root, "state", "service.pid"),
	}
	application := app.New(paths)
	if _, err := application.Tasks.Add(context.Background(), definitionPath, true); err != nil {
		t.Fatal(err)
	}

	server, err := Start(context.Background(), application, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	response, err := http.Get(server.URL() + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "RunBinder") {
		t.Fatalf("dashboard response = %d, %q", response.StatusCode, body)
	}

	response, err = http.Get(server.URL() + "/api/tasks")
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "example.ui") {
		t.Fatalf("task response = %d, %q", response.StatusCode, body)
	}

}
