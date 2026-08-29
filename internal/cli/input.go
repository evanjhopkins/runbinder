package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

func (c *commands) prompt(label, defaultValue string) (string, error) {
	fmt.Fprintf(c.out, "%s [%s]: ", label, defaultValue)
	line, err := c.inputReader().ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func (c *commands) confirm(label string, defaultValue bool) (bool, error) {
	hint := "y/N"
	if defaultValue {
		hint = "Y/n"
	}
	fmt.Fprintf(c.out, "%s [%s]: ", label, hint)
	line, err := c.inputReader().ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultValue, nil
	}
	return line == "y" || line == "yes", nil
}

func (c *commands) inputReader() *bufio.Reader {
	if c.reader == nil {
		c.reader = bufio.NewReader(c.in)
	}
	return c.reader
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", "_", " ", "_").Replace(value)
	if value == "" {
		return "task"
	}
	return value
}
