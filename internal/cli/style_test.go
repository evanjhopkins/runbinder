package cli

import (
	"bytes"
	"testing"
	"time"
)

func TestFormatTimeAlive(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want string
	}{
		{name: "minutes", age: 6*time.Minute + 59*time.Second, want: "0d 0h 6m"},
		{name: "hours", age: 12*time.Hour + 1*time.Minute, want: "0d 12h 1m"},
		{name: "days", age: 1*24*time.Hour + 12*time.Hour + time.Minute, want: "1d 12h 1m"},
		{name: "negative", age: -time.Minute, want: "0d 0h 0m"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatTimeAlive(test.age); got != test.want {
				t.Fatalf("time alive = %q, want %q", got, test.want)
			}
		})
	}
}

func TestServiceStatusLinesRemainPlainForNonTerminalOutput(t *testing.T) {
	commands := &commands{out: &bytes.Buffer{}}
	if got := commands.serviceStatusLine(true, "Is Service Running: TRUE"); got != "Is Service Running: TRUE" {
		t.Fatalf("running status = %q", got)
	}
	if got := commands.serviceStatusLine(false, "Is Service Running: FALSE"); got != "Is Service Running: FALSE" {
		t.Fatalf("stopped status = %q", got)
	}
}

func TestServiceLogLinesRemainPlainForNonTerminalOutput(t *testing.T) {
	commands := &commands{out: &bytes.Buffer{}}
	for _, line := range []string{
		"-> 2026-08-29 RunBinder service started",
		"-> 2026-08-29 RunBinder service stopped",
		"-> 2026-08-29 task completed",
	} {
		if got := commands.serviceLogLine(line); got != line {
			t.Fatalf("log line = %q, want %q", got, line)
		}
	}
}

func TestTaskListLinesRemainPlainForNonTerminalOutput(t *testing.T) {
	commands := &commands{out: &bytes.Buffer{}}
	line := "1  example.task  false  MIS  /tmp/example  (none)"
	row := taskListRow{
		namespace: "example.task", active: "false", workingDir: "/tmp/example", lastRun: "(none)",
	}
	if got := commands.taskListLine(line, row); got != line {
		t.Fatalf("task list line = %q", got)
	}
}
