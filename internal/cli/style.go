package cli

import (
	"io"
	"os"
	"strings"

	"github.com/evanjhopkins/RunBinder/internal/app"
)

const (
	ansiGreen  = "\x1b[32m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiDim    = "\x1b[2m"
	ansiReset  = "\x1b[0m"
)

func (c *commands) serviceStatusLine(running bool, line string) string {
	if running {
		return c.color(ansiGreen, line)
	}
	return c.color(ansiRed, line)
}

func (c *commands) serviceLogLine(line string) string {
	switch {
	case strings.Contains(line, "RunBinder service started"):
		return c.color(ansiGreen, line)
	case strings.Contains(line, "RunBinder service stopped"):
		return c.color(ansiRed, line)
	default:
		return line
	}
}

func (c *commands) color(code, value string) string {
	if !supportsColor(c.out) {
		return value
	}
	return code + value + ansiReset
}

func (c *commands) taskListLine(line string, row taskListRow) string {
	return renderTaskListLine(line, row, supportsColor(c.out))
}

func renderTaskListLine(line string, row taskListRow, useColor bool) string {
	if !useColor {
		return line
	}
	activeStart := strings.Index(line, row.namespace) + len(row.namespace)
	activeStart += strings.Index(line[activeStart:], row.active)
	definitionStart := activeStart + len(row.active)
	definitionStart += strings.Index(line[definitionStart:], string(row.definition))
	lastRunStart := strings.Index(line, row.workingDir) + len(row.workingDir)
	lastRunStart += strings.Index(line[lastRunStart:], row.lastRun)

	activeColor := ansiRed
	if row.enabled {
		activeColor = ansiGreen
	}
	definitionColor := definitionStateColor(row.definition)
	lastRunColor := ""
	if row.hasRun {
		lastRunColor = ansiRed
		if row.success {
			lastRunColor = ansiGreen
		}
	}

	var output strings.Builder
	if !row.enabled {
		output.WriteString(ansiDim)
	}
	spans := []styledSpan{
		{start: activeStart, length: len(row.active), color: activeColor},
		{start: definitionStart, length: len(row.definition), color: definitionColor},
	}
	if lastRunColor != "" {
		spans = append(spans, styledSpan{start: lastRunStart, length: len(row.lastRun), color: lastRunColor})
	}
	last := 0
	for _, span := range spans {
		output.WriteString(line[last:span.start])
		output.WriteString(span.color)
		output.WriteString(line[span.start : span.start+span.length])
		if row.enabled {
			output.WriteString(ansiReset)
		} else {
			output.WriteString(ansiReset + ansiDim)
		}
		last = span.start + span.length
	}
	output.WriteString(line[last:])
	output.WriteString(ansiReset)
	return output.String()
}

func definitionStateColor(state app.DefinitionState) string {
	switch state {
	case app.DefinitionOK:
		return ansiGreen
	case app.DefinitionChanged:
		return ansiYellow
	case app.DefinitionInvalid, app.DefinitionMissing:
		return ansiRed
	default:
		return ansiDim
	}
}

type styledSpan struct {
	start  int
	length int
	color  string
}

func supportsColor(output io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
