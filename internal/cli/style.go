package cli

import (
	"io"
	"os"
	"strings"
)

const (
	ansiGreen = "\x1b[32m"
	ansiRed   = "\x1b[31m"
	ansiReset = "\x1b[0m"
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
