package scheduler

import (
	"testing"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
)

func TestDueCombinesAndDeduplicatesSchedules(t *testing.T) {
	location := time.FixedZone("test", -5*60*60)
	planner := New(location)
	task := domain.Task{
		Namespace:  "example",
		WorkingDir: "/tmp",
		Definition: `namespace: example
command:
  - echo one
  - echo two
schedule:
  time_of_day: ["10:00:00"]
  window_interval:
    start: "10:00:00"
    stop: "10:01:00"
    interval_sec: 60
`,
	}
	after := time.Date(2026, 8, 28, 9, 59, 59, 0, location)
	through := time.Date(2026, 8, 28, 10, 1, 0, 0, location)
	executions, err := planner.Due(task, after, through)
	if err != nil {
		t.Fatal(err)
	}
	if len(executions) != 2 {
		t.Fatalf("got %d executions, want 2", len(executions))
	}
	if executions[0].Command != "echo one && echo two" {
		t.Fatalf("command = %q", executions[0].Command)
	}
	if !executions[0].ScheduledAt.Equal(time.Date(2026, 8, 28, 10, 0, 0, 0, location)) {
		t.Fatalf("first execution = %s", executions[0].ScheduledAt)
	}
}

func TestDueUsesExclusiveStartAndInclusiveEnd(t *testing.T) {
	location := time.UTC
	planner := New(location)
	task := domain.Task{
		Namespace:  "cron-task",
		WorkingDir: "/tmp",
		Definition: "namespace: cron-task\ncommand: echo x\ncron: '* * * * *'\n",
	}
	after := time.Date(2026, 8, 28, 10, 0, 0, 0, location)
	through := time.Date(2026, 8, 28, 10, 2, 0, 0, location)
	executions, err := planner.Due(task, after, through)
	if err != nil {
		t.Fatal(err)
	}
	if len(executions) != 2 {
		t.Fatalf("got %d executions, want 2", len(executions))
	}
	if executions[0].ScheduledAt.Minute() != 1 || executions[1].ScheduledAt.Minute() != 2 {
		t.Fatalf("unexpected execution times: %v", executions)
	}
}

func TestDueSpansMidnight(t *testing.T) {
	planner := New(time.UTC)
	task := domain.Task{
		Namespace:  "daily",
		WorkingDir: "/tmp",
		Definition: "namespace: daily\ncommand: echo x\nschedule:\n  time_of_day: ['00:00:00']\n",
	}
	after := time.Date(2026, 8, 28, 23, 59, 59, 0, time.UTC)
	through := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	executions, err := planner.Due(task, after, through)
	if err != nil {
		t.Fatal(err)
	}
	if len(executions) != 1 || !executions[0].ScheduledAt.Equal(through) {
		t.Fatalf("executions = %#v", executions)
	}
}

func TestDueUsesTaskTimezoneAcrossDST(t *testing.T) {
	planner := New(time.UTC)
	task := domain.Task{
		Namespace:  "new-york-daily",
		WorkingDir: "/tmp",
		Definition: "namespace: new-york-daily\ncommand: echo x\ncron: '0 9 * * *'\ntimezone: America/New_York\n",
	}
	after := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	through := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	executions, err := planner.Due(task, after, through)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Time{
		time.Date(2026, 3, 7, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 8, 13, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC),
	}
	if len(executions) != len(want) {
		t.Fatalf("got %d executions, want %d", len(executions), len(want))
	}
	for index, scheduledAt := range want {
		if !executions[index].ScheduledAt.Equal(scheduledAt) {
			t.Errorf("execution %d = %s, want %s", index, executions[index].ScheduledAt, scheduledAt)
		}
	}
}
