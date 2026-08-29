package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/evanjhopkins/RunBinder/internal/runner"
	"github.com/evanjhopkins/RunBinder/internal/scheduler"
	runbinderservice "github.com/evanjhopkins/RunBinder/internal/service"
)

type ServiceOptions struct {
	Concurrency  int
	MisfireGrace time.Duration
}

type ServiceStatus struct {
	Running    bool
	PID        int
	Heartbeat  *domain.Heartbeat
	StorageDir string
	RecentLogs []string
}

type Service struct {
	paths          platform.Paths
	openRepository OpenRepository
}

func (s *Service) Run(ctx context.Context, options ServiceOptions) error {
	if err := platform.EnsureStorage(s.paths); err != nil {
		return err
	}
	lock, err := runbinderservice.AcquireLock(s.paths.ServiceLock)
	if err != nil {
		return err
	}
	defer lock.Close()
	pid := os.Getpid()
	if err := runbinderservice.WritePID(s.paths.ServicePID, pid); err != nil {
		return err
	}
	defer runbinderservice.RemovePID(s.paths.ServicePID, pid)

	repository, err := s.openRepository()
	if err != nil {
		return err
	}
	defer repository.Close()
	logger := platform.NewLogger(s.paths.InternalLog)
	taskRunner := runner.New(repository, logger)
	service := runbinderservice.New(repository, scheduler.New(time.Local), taskRunner, logger, time.Second, options.MisfireGrace, options.Concurrency)
	return service.Run(ctx)
}

func (s *Service) StartDetached(ctx context.Context, options ServiceOptions) (int, error) {
	if err := platform.EnsureStorage(s.paths); err != nil {
		return 0, err
	}
	probe, err := runbinderservice.AcquireLock(s.paths.ServiceLock)
	if err != nil {
		return 0, err
	}
	if err := probe.Close(); err != nil {
		return 0, err
	}
	if err := runbinderservice.RemovePID(s.paths.ServicePID, 0); err != nil {
		return 0, err
	}
	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("resolve current executable: %w", err)
	}
	args := []string{
		"service", "--detached-child",
		"--concurrency", strconv.Itoa(options.Concurrency),
		"--misfire-grace", options.MisfireGrace.String(),
	}
	startedAt := time.Now()
	pid, err := runbinderservice.StartDetached(executable, args, s.paths.StorageDir, s.paths.InternalLog)
	if err != nil {
		return 0, fmt.Errorf("start detached service: %w", err)
	}

	repository, err := s.openRepository()
	if err != nil {
		_ = runbinderservice.StopProcess(pid)
		return 0, err
	}
	defer repository.Close()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = runbinderservice.StopProcess(pid)
			return 0, ctx.Err()
		case <-timeout.C:
			_ = runbinderservice.StopProcess(pid)
			return 0, errors.New("detached service did not become healthy; check the internal log")
		case <-ticker.C:
			if !runbinderservice.ProcessRunning(pid) {
				return 0, errors.New("detached service exited during startup; check the internal log")
			}
			publishedPID, pidErr := runbinderservice.ReadPID(s.paths.ServicePID)
			heartbeat, heartbeatErr := repository.Heartbeat(ctx, "service")
			if pidErr == nil && publishedPID == pid && heartbeatErr == nil && heartbeat != nil && heartbeat.Running && !heartbeat.Last.Before(startedAt) {
				return pid, nil
			}
		}
	}
}

func (s *Service) Stop(ctx context.Context) (bool, error) {
	if err := platform.EnsureStorage(s.paths); err != nil {
		return false, err
	}
	probe, lockErr := runbinderservice.AcquireLock(s.paths.ServiceLock)
	if lockErr == nil {
		_ = probe.Close()
		_ = runbinderservice.RemovePID(s.paths.ServicePID, 0)
		return false, nil
	}
	pid, err := runbinderservice.ReadPID(s.paths.ServicePID)
	if err != nil {
		return false, fmt.Errorf("service is running but its PID is unavailable: %w", err)
	}
	if !runbinderservice.ProcessRunning(pid) {
		_ = runbinderservice.RemovePID(s.paths.ServicePID, pid)
		return false, nil
	}
	if err := runbinderservice.StopProcess(pid); err != nil {
		return false, fmt.Errorf("stop service: %w", err)
	}

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, errors.New("service did not stop within 5 seconds")
		case <-ticker.C:
			if !runbinderservice.ProcessRunning(pid) {
				_ = runbinderservice.RemovePID(s.paths.ServicePID, pid)
				return true, nil
			}
		}
	}
}

func (s *Service) Status(ctx context.Context) (ServiceStatus, error) {
	repository, err := s.openRepository()
	if err != nil {
		return ServiceStatus{}, err
	}
	defer repository.Close()
	heartbeat, err := repository.Heartbeat(ctx, "service")
	if err != nil {
		return ServiceStatus{}, err
	}
	status := ServiceStatus{
		Running:    heartbeat != nil && heartbeat.Running && time.Since(heartbeat.Last) < 5*time.Second,
		Heartbeat:  heartbeat,
		StorageDir: s.paths.StorageDir,
	}
	if pid, err := runbinderservice.ReadPID(s.paths.ServicePID); err == nil {
		status.PID = pid
	}
	status.RecentLogs, err = platform.Tail(s.paths.InternalLog, 5)
	return status, err
}

func (s *Service) Reset() (bool, error) {
	if err := platform.EnsureStorage(s.paths); err != nil {
		return false, err
	}
	lock, err := runbinderservice.AcquireLock(s.paths.ServiceLock)
	if err != nil {
		return false, fmt.Errorf("cannot delete the database: %w", err)
	}
	defer lock.Close()
	if err := runbinderservice.RemovePID(s.paths.ServicePID, 0); err != nil {
		return false, err
	}
	removed := false
	for _, path := range []string{s.paths.Database, s.paths.Database + "-wal", s.paths.Database + "-shm"} {
		if err := os.Remove(path); err == nil {
			removed = true
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return removed, nil
}
