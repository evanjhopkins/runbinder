package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/evanjhopkins/RunBinder/internal/runner"
	"github.com/evanjhopkins/RunBinder/internal/store"
	"github.com/evanjhopkins/RunBinder/internal/taskconfig"
)

type TaskSummary struct {
	Task       domain.Task
	LastRun    *domain.Run
	Definition DefinitionState
	Timezone   string
}

type DefinitionState string

const (
	DefinitionOK      DefinitionState = "OK"
	DefinitionChanged DefinitionState = "CNG"
	DefinitionInvalid DefinitionState = "INV"
	DefinitionMissing DefinitionState = "MIS"
	DefinitionUnknown DefinitionState = "---"
)

type Tasks struct {
	paths          platform.Paths
	definitions    *Definitions
	openRepository OpenRepository
}

func (t *Tasks) Add(ctx context.Context, path string, active bool) (domain.Task, error) {
	task, err := t.definitions.Load(path)
	if err != nil {
		return domain.Task{}, err
	}
	task.Active = active
	repository, err := t.openRepository()
	if err != nil {
		return domain.Task{}, err
	}
	defer repository.Close()
	return repository.AddTask(ctx, task)
}

func (t *Tasks) Update(ctx context.Context, path string) (domain.Task, error) {
	task, err := t.definitions.Load(path)
	if err != nil {
		return domain.Task{}, err
	}
	repository, err := t.openRepository()
	if err != nil {
		return domain.Task{}, err
	}
	defer repository.Close()
	return repository.UpdateTask(ctx, task)
}

func (t *Tasks) SetActive(ctx context.Context, target string, active bool) (domain.Task, bool, error) {
	repository, err := t.openRepository()
	if err != nil {
		return domain.Task{}, false, err
	}
	defer repository.Close()
	task, err := t.resolve(ctx, repository, target)
	if err != nil {
		return domain.Task{}, false, err
	}
	if task.Active == active {
		return task, false, nil
	}
	if err := repository.SetTaskActive(ctx, task.Namespace, active); err != nil {
		return domain.Task{}, false, err
	}
	task.Active = active
	return task, true, nil
}

func (t *Tasks) Remove(ctx context.Context, target string) (domain.Task, error) {
	repository, err := t.openRepository()
	if err != nil {
		return domain.Task{}, err
	}
	defer repository.Close()
	task, err := t.resolve(ctx, repository, target)
	if err != nil {
		return domain.Task{}, err
	}
	return task, repository.RemoveTask(ctx, task.Namespace)
}

func (t *Tasks) List(ctx context.Context) ([]TaskSummary, error) {
	repository, err := t.openRepository()
	if err != nil {
		return nil, err
	}
	defer repository.Close()
	tasks, err := repository.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]TaskSummary, 0, len(tasks))
	for _, task := range tasks {
		lastRun, err := repository.LastRun(ctx, task.Namespace)
		if err != nil {
			return nil, err
		}
		timezone, err := taskTimezone(task)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, TaskSummary{
			Task:       task,
			LastRun:    lastRun,
			Definition: t.definitionState(task),
			Timezone:   timezone,
		})
	}
	return summaries, nil
}

func taskTimezone(task domain.Task) (string, error) {
	cfg, err := taskconfig.Parse([]byte(task.Definition))
	if err != nil {
		return "", fmt.Errorf("task %q: %w", task.Namespace, err)
	}
	if cfg.Timezone == "" {
		return "LOCAL", nil
	}
	return cfg.Timezone, nil
}

func (t *Tasks) definitionState(task domain.Task) DefinitionState {
	if task.SourcePath == "" {
		return DefinitionUnknown
	}
	hash, err := t.definitions.Hash(task.SourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return DefinitionMissing
	}
	if err != nil {
		return DefinitionInvalid
	}
	if hash != task.Hash {
		return DefinitionChanged
	}
	return DefinitionOK
}

func (t *Tasks) Log(ctx context.Context, target string, lines int) ([]string, error) {
	repository, err := t.openRepository()
	if err != nil {
		return nil, err
	}
	defer repository.Close()
	task, err := t.resolve(ctx, repository, target)
	if err != nil {
		return nil, err
	}
	return platform.Tail(filepath.Join(task.WorkingDir, platform.TaskLogName), lines)
}

func (t *Tasks) Run(ctx context.Context, target string) (domain.Task, error) {
	repository, err := t.openRepository()
	if err != nil {
		return domain.Task{}, err
	}
	defer repository.Close()
	task, err := t.resolve(ctx, repository, target)
	if err != nil {
		return domain.Task{}, err
	}
	cfg, err := taskconfig.Parse([]byte(task.Definition))
	if err != nil {
		return domain.Task{}, err
	}
	execution := domain.Execution{
		Namespace: task.Namespace, ScheduledAt: time.Now(),
		Command: cfg.Command.Shell(), WorkingDir: task.WorkingDir,
	}
	if err := runner.New(repository, platform.NewLogger(t.paths.InternalLog)).Run(ctx, execution); err != nil {
		return domain.Task{}, fmt.Errorf("task %q failed: %w", task.Namespace, err)
	}
	return task, nil
}

func (t *Tasks) resolve(ctx context.Context, repository store.Repository, target string) (domain.Task, error) {
	namespace := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		namespace, err = t.definitions.Namespace(target)
		if err != nil {
			return domain.Task{}, err
		}
	}
	task, err := repository.Task(ctx, namespace)
	if errors.Is(err, store.ErrNotFound) {
		return domain.Task{}, errors.New("unable to resolve task; provide a registered namespace or task file")
	}
	return task, err
}
