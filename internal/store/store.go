package store

import (
	"context"
	"errors"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
)

var (
	ErrNotFound      = errors.New("task not found")
	ErrAlreadyExists = errors.New("task namespace already exists")
	ErrNoChanges     = errors.New("task definition has not changed")
)

type Repository interface {
	AddTask(context.Context, domain.Task) (domain.Task, error)
	UpdateTask(context.Context, domain.Task) (domain.Task, error)
	Task(context.Context, string) (domain.Task, error)
	ListTasks(context.Context) ([]domain.Task, error)
	ListEnabledTasks(context.Context) ([]domain.Task, error)
	SetTaskActive(context.Context, string, bool) error
	RemoveTask(context.Context, string) error
	LastRun(context.Context, string) (*domain.Run, error)
	RecordRun(context.Context, domain.Run) error
	UpdateHeartbeat(context.Context, string, time.Time) error
	StopHeartbeat(context.Context, string, time.Time) error
	Heartbeat(context.Context, string) (*domain.Heartbeat, error)
	Close() error
}
