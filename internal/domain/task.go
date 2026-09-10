package domain

import "time"

type Task struct {
	ID         int64
	Namespace  string
	Active     bool
	Definition string
	Hash       string
	WorkingDir string
	SourcePath string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Run struct {
	ID          int64
	Namespace   string
	ScheduledAt time.Time
	StartedAt   time.Time
	FinishedAt  time.Time
	Success     bool
	Error       string
}

type Heartbeat struct {
	Last      time.Time
	StartedAt time.Time
	Running   bool
}

type Execution struct {
	Namespace    string
	ScheduledAt  time.Time
	Command      string
	WorkingDir   string
	AllowOverlap bool
}
