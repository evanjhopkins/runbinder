package scheduler

import (
	"fmt"
	"sort"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/taskconfig"
)

type Planner struct {
	location *time.Location
}

func New(location *time.Location) *Planner {
	if location == nil {
		location = time.Local
	}
	return &Planner{location: location}
}

// Due returns occurrences in the half-open progression (after, through].
// This boundary lets the service advance its cursor without duplicate runs.
func (p *Planner) Due(task domain.Task, after, through time.Time) ([]domain.Execution, error) {
	if !through.After(after) {
		return nil, nil
	}
	cfg, err := taskconfig.Parse([]byte(task.Definition))
	if err != nil {
		return nil, fmt.Errorf("task %q: %w", task.Namespace, err)
	}
	location, err := cfg.Location(p.location)
	if err != nil {
		return nil, fmt.Errorf("task %q: %w", task.Namespace, err)
	}

	times := make(map[int64]time.Time)
	add := func(value time.Time) {
		if value.After(after) && !value.After(through) {
			times[value.UnixNano()] = value
		}
	}

	if cfg.Cron != "" {
		schedule, err := taskconfig.ParseCron(cfg.Cron)
		if err != nil {
			return nil, err
		}
		for next := schedule.Next(after.In(location)); !next.After(through); next = schedule.Next(next) {
			add(next)
		}
	}

	if cfg.Schedule != nil {
		firstDate := midnight(after.In(location))
		lastDate := midnight(through.In(location))
		for date := firstDate; !date.After(lastDate); date = date.AddDate(0, 0, 1) {
			for _, raw := range cfg.Schedule.TimeOfDay {
				clock, _ := taskconfig.ParseClock(raw)
				add(onDate(date, clock))
			}
			if window := cfg.Schedule.WindowInterval; window != nil {
				start, _ := taskconfig.ParseClock(window.Start)
				stop, _ := taskconfig.ParseClock(window.Stop)
				current := onDate(date, start)
				end := onDate(date, stop)
				step := time.Duration(window.IntervalSec) * time.Second
				for !current.After(end) {
					add(current)
					current = current.Add(step)
				}
			}
		}
	}

	ordered := make([]time.Time, 0, len(times))
	for _, value := range times {
		ordered = append(ordered, value)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Before(ordered[j]) })

	executions := make([]domain.Execution, 0, len(ordered))
	for _, scheduledAt := range ordered {
		executions = append(executions, domain.Execution{
			Namespace:    task.Namespace,
			ScheduledAt:  scheduledAt,
			Command:      cfg.Command.Shell(),
			WorkingDir:   task.WorkingDir,
			AllowOverlap: cfg.AllowOverlap,
		})
	}
	return executions, nil
}

func midnight(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func onDate(date, clock time.Time) time.Time {
	year, month, day := date.Date()
	hour, minute, second := clock.Clock()
	return time.Date(year, month, day, hour, minute, second, 0, date.Location())
}
