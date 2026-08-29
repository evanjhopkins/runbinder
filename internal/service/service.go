package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/evanjhopkins/RunBinder/internal/runner"
	"github.com/evanjhopkins/RunBinder/internal/scheduler"
	"github.com/evanjhopkins/RunBinder/internal/store"
)

type Service struct {
	repository     store.Repository
	planner        *scheduler.Planner
	runner         *runner.Runner
	logger         *platform.Logger
	tickInterval   time.Duration
	misfireGrace   time.Duration
	maxConcurrency int
	runningMu      sync.Mutex
	running        map[string]int
}

func New(repository store.Repository, planner *scheduler.Planner, taskRunner *runner.Runner, logger *platform.Logger, tickInterval, misfireGrace time.Duration, maxConcurrency int) *Service {
	if tickInterval <= 0 {
		tickInterval = time.Second
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	if misfireGrace < tickInterval {
		misfireGrace = tickInterval
	}
	return &Service{
		repository: repository, planner: planner, runner: taskRunner, logger: logger,
		tickInterval: tickInterval, misfireGrace: misfireGrace, maxConcurrency: maxConcurrency,
		running: make(map[string]int),
	}
}

func (s *Service) Run(ctx context.Context) error {
	if err := s.logger.Write("RunBinder service started"); err != nil {
		return fmt.Errorf("write service log: %w", err)
	}
	defer func() {
		_ = s.repository.StopHeartbeat(context.Background(), "service", time.Now())
	}()
	if err := s.repository.UpdateHeartbeat(ctx, "service", time.Now()); err != nil {
		return err
	}

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()
	semaphore := make(chan struct{}, s.maxConcurrency)
	var active sync.WaitGroup
	lastTick := time.Now()

	for {
		select {
		case <-ctx.Done():
			active.Wait()
			_ = s.logger.Write("RunBinder service stopped")
			return nil
		case now := <-ticker.C:
			if now.Before(lastTick) {
				lastTick = now
				continue
			}
			if err := s.repository.UpdateHeartbeat(ctx, "service", now); err != nil {
				_ = s.logger.Write(fmt.Sprintf("heartbeat failed: %v", err))
			}
			tasks, err := s.repository.ListEnabledTasks(ctx)
			if err != nil {
				_ = s.logger.Write(fmt.Sprintf("load tasks failed: %v", err))
				lastTick = now
				continue
			}
			after := lastTick
			if earliest := now.Add(-s.misfireGrace); after.Before(earliest) {
				after = earliest
				_ = s.logger.Write(fmt.Sprintf("scheduler resumed after a gap; skipped occurrences before %s", earliest.Format(time.RFC3339)))
			}
			s.dispatch(ctx, &active, semaphore, tasks, after, now)
			lastTick = now
		}
	}
}

func (s *Service) dispatch(ctx context.Context, active *sync.WaitGroup, semaphore chan struct{}, tasks []domain.Task, after, through time.Time) {
	for _, task := range tasks {
		executions, err := s.planner.Due(task, after, through)
		if err != nil {
			_ = s.logger.Write(fmt.Sprintf("schedule %s failed: %v", task.Namespace, err))
			continue
		}
		for _, execution := range executions {
			if ctx.Err() != nil {
				return
			}
			if !s.reserve(execution.Namespace, execution.AllowOverlap) {
				_ = s.logger.Write(fmt.Sprintf("skipped overlapping run for %s scheduled at %s", execution.Namespace, execution.ScheduledAt.Format(time.RFC3339)))
				continue
			}
			active.Add(1)
			go func(execution domain.Execution) {
				defer active.Done()
				defer s.release(execution.Namespace)
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
					s.runner.Run(ctx, execution)
				case <-ctx.Done():
				}
			}(execution)
		}
	}
}

func (s *Service) reserve(namespace string, allowOverlap bool) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	if !allowOverlap && s.running[namespace] > 0 {
		return false
	}
	s.running[namespace]++
	return true
}

func (s *Service) release(namespace string) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	s.running[namespace]--
	if s.running[namespace] == 0 {
		delete(s.running, namespace)
	}
}
