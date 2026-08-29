//go:build !windows

package runner

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/evanjhopkins/RunBinder/internal/store"
)

func TestRunnerCapturesOutputAndRunStatus(t *testing.T) {
	root := t.TempDir()
	repository, err := store.OpenSQLite(filepath.Join(root, "runbinder.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	logger := platform.NewLogger(filepath.Join(root, "internal.log"))
	taskRunner := New(repository, logger)

	execution := domain.Execution{
		Namespace: "example", ScheduledAt: time.Now(),
		Command: "printf 'hello from task\\n'", WorkingDir: root,
	}
	taskRunner.Run(context.Background(), execution)

	lines, err := platform.Tail(filepath.Join(root, platform.TaskLogName), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "hello from task" {
		t.Fatalf("task log = %#v", lines)
	}
	run, err := repository.LastRun(context.Background(), "example")
	if err != nil || run == nil || !run.Success {
		t.Fatalf("run = %#v, err = %v", run, err)
	}

	execution.Command = "exit 7"
	taskRunner.Run(context.Background(), execution)
	run, err = repository.LastRun(context.Background(), "example")
	if err != nil || run == nil || run.Success || !strings.Contains(run.Error, "exit status 7") {
		t.Fatalf("failed run = %#v, err = %v", run, err)
	}
}
