package cli

import (
	"strings"
	"testing"
)

func TestTaskListLineRestoresDefaultForegroundForInactiveRows(t *testing.T) {
	line := "1  example.task  false  /tmp/example  2026-08-29 (FAIL)"
	row := taskListRow{
		namespace:  "example.task",
		active:     "false",
		workingDir: "/tmp/example",
		lastRun:    "2026-08-29 (FAIL)",
		hasRun:     true,
	}
	styled := renderTaskListLine(line, row, true)
	want := ansiRed + "false" + ansiReset + ansiDim
	if !strings.Contains(styled, want) {
		t.Fatalf("directory did not restore the default foreground: %q", styled)
	}
}
