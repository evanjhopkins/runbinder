package cli

import (
	"bytes"
	"testing"
)

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
