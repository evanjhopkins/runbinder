package cli

import (
	"strings"
	"testing"

	"github.com/evanjhopkins/RunBinder/internal/app"
)

func TestTaskListLineRestoresDefaultForegroundForInactiveRows(t *testing.T) {
	line := "1  example.task  false  MIS  /tmp/example  2026-08-29 (FAIL)"
	row := taskListRow{
		namespace:  "example.task",
		active:     "false",
		definition: app.DefinitionMissing,
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

func TestDefinitionStateColors(t *testing.T) {
	tests := []struct {
		state app.DefinitionState
		want  string
	}{
		{app.DefinitionOK, ansiGreen},
		{app.DefinitionChanged, ansiYellow},
		{app.DefinitionInvalid, ansiRed},
		{app.DefinitionMissing, ansiRed},
	}
	for _, test := range tests {
		if got := definitionStateColor(test.state); got != test.want {
			t.Errorf("color for %q = %q, want %q", test.state, got, test.want)
		}
	}
}
