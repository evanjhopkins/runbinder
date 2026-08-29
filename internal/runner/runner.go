package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/evanjhopkins/RunBinder/internal/store"
)

type Runner struct {
	repository store.Repository
	logger     *platform.Logger
}

func New(repository store.Repository, logger *platform.Logger) *Runner {
	return &Runner{repository: repository, logger: logger}
}

func (r *Runner) Run(ctx context.Context, execution domain.Execution) error {
	run := domain.Run{
		Namespace:   execution.Namespace,
		ScheduledAt: execution.ScheduledAt,
		StartedAt:   time.Now(),
	}

	err := execute(ctx, execution)
	run.FinishedAt = time.Now()
	run.Success = err == nil
	if err != nil {
		run.Error = err.Error()
		_ = r.logger.Write(fmt.Sprintf("%s FAILED: %v", execution.Namespace, err))
	}
	if recordErr := r.repository.RecordRun(context.Background(), run); recordErr != nil {
		_ = r.logger.Write(fmt.Sprintf("could not record run for %s: %v", execution.Namespace, recordErr))
		if err == nil {
			return recordErr
		}
	}
	return err
}

func execute(ctx context.Context, execution domain.Execution) error {
	logPath := filepath.Join(execution.WorkingDir, platform.TaskLogName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open task log: %w", err)
	}
	defer logFile.Close()

	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.CommandContext(ctx, "cmd.exe", "/C", execution.Command)
	} else {
		command = exec.CommandContext(ctx, "/bin/sh", "-c", execution.Command)
	}
	command.Dir = execution.WorkingDir
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Run(); err != nil {
		return fmt.Errorf("execute command: %w", err)
	}
	return nil
}
