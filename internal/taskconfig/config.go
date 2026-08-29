package taskconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

type Command []string

func (c *Command) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return err
		}
		*c = Command{value}
	case yaml.SequenceNode:
		var values []string
		if err := node.Decode(&values); err != nil {
			return errors.New("command must contain only strings")
		}
		*c = values
	default:
		return errors.New("command must be a string or a list of strings")
	}
	return nil
}

func (c Command) Shell() string {
	return strings.Join(c, " && ")
}

type WindowInterval struct {
	Start       string `yaml:"start"`
	Stop        string `yaml:"stop"`
	IntervalSec int    `yaml:"interval_sec"`
}

type Schedule struct {
	TimeOfDay      []string        `yaml:"time_of_day"`
	WindowInterval *WindowInterval `yaml:"window_interval"`
}

type Config struct {
	Namespace    string    `yaml:"namespace"`
	Command      Command   `yaml:"command"`
	Cron         string    `yaml:"cron,omitempty"`
	Schedule     *Schedule `yaml:"schedule,omitempty"`
	WorkingDir   string    `yaml:"working_dir,omitempty"`
	Timezone     string    `yaml:"timezone,omitempty"`
	AllowOverlap bool      `yaml:"allow_overlap,omitempty"`
}

var cronParser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

func Parse(data []byte) (Config, error) {
	var cfg Config
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse task definition: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Namespace) == "" {
		return errors.New("namespace is required")
	}
	if strings.ContainsAny(c.Namespace, "\r\n\t") {
		return errors.New("namespace cannot contain whitespace control characters")
	}
	if len(c.Command) == 0 {
		return errors.New("command is required")
	}
	for _, command := range c.Command {
		if strings.TrimSpace(command) == "" {
			return errors.New("command entries cannot be empty")
		}
	}

	hasSchedule := false
	if c.Cron != "" {
		hasSchedule = true
		if _, err := cronParser.Parse(c.Cron); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	}
	if c.Schedule != nil {
		for _, value := range c.Schedule.TimeOfDay {
			hasSchedule = true
			if _, err := parseClock(value); err != nil {
				return fmt.Errorf("invalid time_of_day %q: %w", value, err)
			}
		}
		if window := c.Schedule.WindowInterval; window != nil {
			hasSchedule = true
			start, err := parseClock(window.Start)
			if err != nil {
				return fmt.Errorf("invalid window start: %w", err)
			}
			stop, err := parseClock(window.Stop)
			if err != nil {
				return fmt.Errorf("invalid window stop: %w", err)
			}
			if stop.Before(start) {
				return errors.New("window stop must not be before start")
			}
			if window.IntervalSec <= 0 {
				return errors.New("window interval_sec must be greater than zero")
			}
		}
	}
	if !hasSchedule {
		return errors.New("at least one cron or schedule entry is required")
	}
	if _, err := c.Location(time.Local); err != nil {
		return err
	}
	return nil
}

func (c Config) Location(defaultLocation *time.Location) (*time.Location, error) {
	if c.Timezone == "" {
		if defaultLocation == nil {
			return time.Local, nil
		}
		return defaultLocation, nil
	}
	location, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", c.Timezone, err)
	}
	return location, nil
}

func (c Config) Canonical() (string, string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", "", fmt.Errorf("encode task definition: %w", err)
	}
	sum := sha256.Sum256(data)
	return string(data), hex.EncodeToString(sum[:]), nil
}

func ParseCron(expression string) (cron.Schedule, error) {
	return cronParser.Parse(expression)
}

func ParseClock(value string) (time.Time, error) {
	return parseClock(value)
}

func parseClock(value string) (time.Time, error) {
	return time.Parse("15:04:05", value)
}
