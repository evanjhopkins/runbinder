package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/evanjhopkins/RunBinder/internal/runner"
	"github.com/evanjhopkins/RunBinder/internal/scheduler"
	runbinderservice "github.com/evanjhopkins/RunBinder/internal/service"
	"github.com/evanjhopkins/RunBinder/internal/store"
	"github.com/evanjhopkins/RunBinder/internal/taskconfig"
	"github.com/spf13/cobra"
)

const version = "0.2.0"

type application struct {
	paths  platform.Paths
	in     io.Reader
	out    io.Writer
	reader *bufio.Reader
}

func Execute() error {
	paths, err := platform.ResolvePaths()
	if err != nil {
		return err
	}
	app := &application{paths: paths, in: os.Stdin, out: os.Stdout, reader: bufio.NewReader(os.Stdin)}
	return app.rootCommand().Execute()
}

func (a *application) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           filepath.Base(os.Args[0]),
		Short:         "File-based scheduled process execution",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(a.in)
	root.SetOut(a.out)
	root.AddCommand(
		a.addCommand(),
		a.updateCommand(),
		a.stateCommand("enable", true),
		a.stateCommand("disable", false),
		a.removeCommand(),
		a.listCommand(),
		a.statusCommand(),
		a.logCommand(),
		a.runCommand(),
		a.serviceCommand(),
		a.initCommand(),
		a.nukeCommand(),
	)
	return root
}

func (a *application) repository() (*store.SQLite, error) {
	if err := platform.EnsureStorage(a.paths); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}
	return store.OpenSQLite(a.paths.Database)
}

func (a *application) addCommand() *cobra.Command {
	var enabled bool
	command := &cobra.Command{
		Use:   "add <task-file>",
		Short: "Register a task definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, err := taskFromFile(args[0])
			if err != nil {
				return err
			}
			task.Active = enabled
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			if _, err := repository.AddTask(cmd.Context(), task); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "[RUNBINDER] Registered task %q.\n", task.Namespace)
			return nil
		},
	}
	command.Flags().BoolVarP(&enabled, "enable", "e", false, "enable the task immediately")
	return command
}

func (a *application) updateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update <task-file>",
		Short: "Update a registered task from its definition file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task, err := taskFromFile(args[0])
			if err != nil {
				return err
			}
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			if _, err := repository.UpdateTask(cmd.Context(), task); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "[RUNBINDER] Updated task %q.\n", task.Namespace)
			return nil
		},
	}
}

func (a *application) stateCommand(name string, active bool) *cobra.Command {
	verb := "Enable"
	if !active {
		verb = "Disable"
	}
	return &cobra.Command{
		Use:   name + " <namespace-or-task-file>",
		Short: verb + " a registered task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			task, err := resolveTask(cmd.Context(), repository, args[0])
			if err != nil {
				return err
			}
			if task.Active == active {
				fmt.Fprintf(a.out, "[RUNBINDER] Task %q is already %s.\n", task.Namespace, stateName(active))
				return nil
			}
			if err := repository.SetTaskActive(cmd.Context(), task.Namespace, active); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "[RUNBINDER] Task %q is now %s.\n", task.Namespace, stateName(active))
			return nil
		},
	}
}

func (a *application) removeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <namespace-or-task-file>",
		Short: "Remove a task registration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			task, err := resolveTask(cmd.Context(), repository, args[0])
			if err != nil {
				return err
			}
			if err := repository.RemoveTask(cmd.Context(), task.Namespace); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "[RUNBINDER] Removed task %q.\n", task.Namespace)
			return nil
		},
	}
}

func (a *application) listCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			tasks, err := repository.ListTasks(cmd.Context())
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(a.out, "[RUNBINDER] No tasks have been registered.")
				return nil
			}
			writer := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tNAMESPACE\tACTIVE\tDIRECTORY\tLAST RUN")
			for _, task := range tasks {
				lastRun := "(none)"
				run, err := repository.LastRun(cmd.Context(), task.Namespace)
				if err != nil {
					return err
				}
				if run != nil {
					status := "FAIL"
					if run.Success {
						status = "SUCC"
					}
					lastRun = run.StartedAt.Local().Format("2006-01-02 15:04:05") + " (" + status + ")"
				}
				fmt.Fprintf(writer, "%d\t%s\t%t\t%s\t%s\n", task.ID, task.Namespace, task.Active, task.WorkingDir, lastRun)
			}
			return writer.Flush()
		},
	}
}

func (a *application) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show service health and recent internal logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			heartbeat, err := repository.Heartbeat(cmd.Context(), "service")
			if err != nil {
				return err
			}
			running := heartbeat != nil && heartbeat.Running && time.Since(heartbeat.Last) < 5*time.Second
			last := "(none)"
			pid := "(none)"
			if heartbeat != nil {
				last = heartbeat.Last.Local().Format("2006-01-02 15:04:05")
			}
			if servicePID, err := runbinderservice.ReadPID(a.paths.ServicePID); err == nil {
				pid = strconv.Itoa(servicePID)
			}
			fmt.Fprintf(a.out, "Is Service Running: %s\n", strings.ToUpper(strconv.FormatBool(running)))
			fmt.Fprintf(a.out, "Service PID: %s\n", pid)
			fmt.Fprintf(a.out, "Last Heartbeat: %s\n", last)
			fmt.Fprintf(a.out, "Internal Storage: %s\n", a.paths.StorageDir)
			fmt.Fprintln(a.out, "Recent Logs:")
			lines, err := platform.Tail(a.paths.InternalLog, 5)
			if err != nil {
				return err
			}
			if len(lines) == 0 {
				fmt.Fprintln(a.out, "(none)")
			}
			for _, line := range lines {
				fmt.Fprintln(a.out, "-> "+line)
			}
			return nil
		},
	}
}

func (a *application) logCommand() *cobra.Command {
	var lines int
	command := &cobra.Command{
		Use:   "log <namespace-or-task-file>",
		Short: "Show recent output from a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			task, err := resolveTask(cmd.Context(), repository, args[0])
			if err != nil {
				return err
			}
			entries, err := platform.Tail(filepath.Join(task.WorkingDir, platform.TaskLogName), lines)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(a.out, "(none)")
			}
			for _, entry := range entries {
				fmt.Fprintln(a.out, entry)
			}
			return nil
		},
	}
	command.Flags().IntVarP(&lines, "lines", "n", 20, "number of lines to show")
	return command
}

func (a *application) runCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run <namespace-or-task-file>",
		Short: "Run a task immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			task, err := resolveTask(cmd.Context(), repository, args[0])
			if err != nil {
				return err
			}
			cfg, err := taskconfig.Parse([]byte(task.Definition))
			if err != nil {
				return err
			}
			now := time.Now()
			execution := domain.Execution{
				Namespace: task.Namespace, ScheduledAt: now,
				Command: cfg.Command.Shell(), WorkingDir: task.WorkingDir,
			}
			logger := platform.NewLogger(a.paths.InternalLog)
			if err := runner.New(repository, logger).Run(cmd.Context(), execution); err != nil {
				return fmt.Errorf("task %q failed: %w", task.Namespace, err)
			}
			fmt.Fprintf(a.out, "[RUNBINDER] Task %q completed successfully.\n", task.Namespace)
			return nil
		},
	}
}

func (a *application) serviceCommand() *cobra.Command {
	var concurrency int
	var misfireGrace time.Duration
	var detach bool
	var detachedChild bool
	command := &cobra.Command{
		Use:   "service",
		Short: "Run the RunBinder scheduling service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if concurrency < 1 {
				return errors.New("concurrency must be at least 1")
			}
			if detach && detachedChild {
				return errors.New("detach and detached-child cannot be used together")
			}
			if detach {
				return a.startDetachedService(cmd.Context(), concurrency, misfireGrace)
			}
			return a.runService(cmd.Context(), concurrency, misfireGrace, detachedChild)
		},
	}
	command.Flags().IntVarP(&concurrency, "concurrency", "j", 4, "maximum number of tasks to run concurrently")
	command.Flags().DurationVar(&misfireGrace, "misfire-grace", time.Minute, "maximum age of a delayed occurrence to run")
	command.Flags().BoolVarP(&detach, "detach", "d", false, "run the service in the background")
	command.Flags().BoolVar(&detachedChild, "detached-child", false, "internal detached service mode")
	_ = command.Flags().MarkHidden("detached-child")
	command.AddCommand(a.stopServiceCommand())
	return command
}

func (a *application) runService(ctx context.Context, concurrency int, misfireGrace time.Duration, detachedChild bool) error {
	if err := platform.EnsureStorage(a.paths); err != nil {
		return err
	}
	lock, err := runbinderservice.AcquireLock(a.paths.ServiceLock)
	if err != nil {
		return err
	}
	defer lock.Close()
	pid := os.Getpid()
	if err := runbinderservice.WritePID(a.paths.ServicePID, pid); err != nil {
		return err
	}
	defer runbinderservice.RemovePID(a.paths.ServicePID, pid)

	repository, err := a.repository()
	if err != nil {
		return err
	}
	defer repository.Close()
	logger := platform.NewLogger(a.paths.InternalLog)
	taskRunner := runner.New(repository, logger)
	service := runbinderservice.New(repository, scheduler.New(time.Local), taskRunner, logger, time.Second, misfireGrace, concurrency)
	serviceCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !detachedChild {
		fmt.Fprintln(a.out, "RunBinder service started. Press Ctrl+C to stop.")
	}
	return service.Run(serviceCtx)
}

func (a *application) startDetachedService(ctx context.Context, concurrency int, misfireGrace time.Duration) error {
	if err := platform.EnsureStorage(a.paths); err != nil {
		return err
	}
	probe, err := runbinderservice.AcquireLock(a.paths.ServiceLock)
	if err != nil {
		return err
	}
	if err := probe.Close(); err != nil {
		return err
	}
	if err := runbinderservice.RemovePID(a.paths.ServicePID, 0); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	args := []string{
		"service", "--detached-child",
		"--concurrency", strconv.Itoa(concurrency),
		"--misfire-grace", misfireGrace.String(),
	}
	startedAt := time.Now()
	pid, err := runbinderservice.StartDetached(executable, args, a.paths.StorageDir, a.paths.InternalLog)
	if err != nil {
		return fmt.Errorf("start detached service: %w", err)
	}

	repository, err := a.repository()
	if err != nil {
		_ = runbinderservice.StopProcess(pid)
		return err
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
			return ctx.Err()
		case <-timeout.C:
			_ = runbinderservice.StopProcess(pid)
			return errors.New("detached service did not become healthy; check the internal log")
		case <-ticker.C:
			if !runbinderservice.ProcessRunning(pid) {
				return errors.New("detached service exited during startup; check the internal log")
			}
			publishedPID, pidErr := runbinderservice.ReadPID(a.paths.ServicePID)
			heartbeat, heartbeatErr := repository.Heartbeat(ctx, "service")
			if pidErr == nil && publishedPID == pid && heartbeatErr == nil && heartbeat != nil && heartbeat.Running && !heartbeat.Last.Before(startedAt) {
				fmt.Fprintf(a.out, "[RUNBINDER] Service started in the background (PID %d).\n", pid)
				return nil
			}
		}
	}
}

func (a *application) stopServiceCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := platform.EnsureStorage(a.paths); err != nil {
				return err
			}
			probe, lockErr := runbinderservice.AcquireLock(a.paths.ServiceLock)
			if lockErr == nil {
				_ = probe.Close()
				_ = runbinderservice.RemovePID(a.paths.ServicePID, 0)
				fmt.Fprintln(a.out, "[RUNBINDER] Service is not running.")
				return nil
			}
			pid, err := runbinderservice.ReadPID(a.paths.ServicePID)
			if err != nil {
				return fmt.Errorf("service is running but its PID is unavailable: %w", err)
			}
			if !runbinderservice.ProcessRunning(pid) {
				_ = runbinderservice.RemovePID(a.paths.ServicePID, pid)
				fmt.Fprintln(a.out, "[RUNBINDER] Service is not running.")
				return nil
			}
			if err := runbinderservice.StopProcess(pid); err != nil {
				return fmt.Errorf("stop service: %w", err)
			}

			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-cmd.Context().Done():
					return cmd.Context().Err()
				case <-deadline.C:
					return errors.New("service did not stop within 5 seconds")
				case <-ticker.C:
					if !runbinderservice.ProcessRunning(pid) {
						_ = runbinderservice.RemovePID(a.paths.ServicePID, pid)
						fmt.Fprintln(a.out, "[RUNBINDER] Service stopped.")
						return nil
					}
				}
			}
		},
	}
}

func (a *application) initCommand() *cobra.Command {
	var register, enable, force bool
	command := &cobra.Command{
		Use:   "init",
		Short: "Create a task definition in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			name := normalizeName(filepath.Base(cwd))
			path := filepath.Join(cwd, name+".runbinder.yaml")
			if _, err := os.Stat(path); err == nil && !force {
				overwrite, err := a.confirm(fmt.Sprintf("File %s exists. Overwrite?", filepath.Base(path)), false)
				if err != nil || !overwrite {
					return err
				}
			}
			defaultNamespace := "myproject." + name + ".task1"
			namespace, err := a.prompt("Namespace", defaultNamespace)
			if err != nil {
				return err
			}
			cfg := taskconfig.Config{
				Namespace: namespace,
				Command:   taskconfig.Command{`echo "Running RunBinder task"`, `echo "init task"`},
				Schedule: &taskconfig.Schedule{WindowInterval: &taskconfig.WindowInterval{
					Start: "00:00:00", Stop: "23:59:59", IntervalSec: 60,
				}},
				WorkingDir: cwd,
			}
			definition, _, err := cfg.Canonical()
			if err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(definition), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "[RUNBINDER] Created %s.\n", path)

			shouldRegister := register
			if !cmd.Flags().Changed("register") {
				shouldRegister, err = a.confirm("Register this task?", true)
				if err != nil {
					return err
				}
			}
			if !shouldRegister {
				return nil
			}
			task, err := taskFromFile(path)
			if err != nil {
				return err
			}
			task.Active = enable
			repository, err := a.repository()
			if err != nil {
				return err
			}
			defer repository.Close()
			if _, err := repository.AddTask(cmd.Context(), task); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "[RUNBINDER] Registered task %q.\n", task.Namespace)
			return nil
		},
	}
	command.Flags().BoolVar(&register, "register", false, "register the generated task without prompting")
	command.Flags().BoolVarP(&enable, "enable", "e", false, "enable the task when registering")
	command.Flags().BoolVarP(&force, "force", "f", false, "overwrite an existing definition")
	return command
}

func (a *application) nukeCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "nuke",
		Short: "Delete RunBinder's database and all registrations",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !yes {
				confirmed, err := a.confirm("Delete all task registrations and run history?", false)
				if err != nil || !confirmed {
					return err
				}
			}
			if err := platform.EnsureStorage(a.paths); err != nil {
				return err
			}
			lock, err := runbinderservice.AcquireLock(a.paths.ServiceLock)
			if err != nil {
				return fmt.Errorf("cannot delete the database: %w", err)
			}
			defer lock.Close()
			if err := runbinderservice.RemovePID(a.paths.ServicePID, 0); err != nil {
				return err
			}
			removed := false
			for _, path := range []string{a.paths.Database, a.paths.Database + "-wal", a.paths.Database + "-shm"} {
				if err := os.Remove(path); err == nil {
					removed = true
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			if removed {
				fmt.Fprintln(a.out, "[RUNBINDER] Deleted the RunBinder database.")
			} else {
				fmt.Fprintln(a.out, "[RUNBINDER] No database to delete.")
			}
			return nil
		},
	}
	command.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return command
}

func taskFromFile(path string) (domain.Task, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return domain.Task{}, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return domain.Task{}, fmt.Errorf("read task definition: %w", err)
	}
	cfg, err := taskconfig.Parse(data)
	if err != nil {
		return domain.Task{}, err
	}
	definition, hash, err := cfg.Canonical()
	if err != nil {
		return domain.Task{}, err
	}
	workingDir := filepath.Dir(abs)
	if cfg.WorkingDir != "" {
		workingDir = cfg.WorkingDir
		if !filepath.IsAbs(workingDir) {
			workingDir = filepath.Join(filepath.Dir(abs), workingDir)
		}
		workingDir, err = filepath.Abs(workingDir)
		if err != nil {
			return domain.Task{}, err
		}
	}
	info, err := os.Stat(workingDir)
	if err != nil {
		return domain.Task{}, fmt.Errorf("working directory: %w", err)
	}
	if !info.IsDir() {
		return domain.Task{}, errors.New("working_dir is not a directory")
	}
	return domain.Task{
		Namespace: cfg.Namespace, Definition: definition, Hash: hash,
		WorkingDir: workingDir, SourcePath: abs,
	}, nil
}

func resolveTask(ctx context.Context, repository store.Repository, target string) (domain.Task, error) {
	namespace := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		data, err := os.ReadFile(target)
		if err != nil {
			return domain.Task{}, err
		}
		cfg, err := taskconfig.Parse(data)
		if err != nil {
			return domain.Task{}, err
		}
		namespace = cfg.Namespace
	}
	task, err := repository.Task(ctx, namespace)
	if errors.Is(err, store.ErrNotFound) {
		return domain.Task{}, errors.New("unable to resolve task; provide a registered namespace or task file")
	}
	return task, err
}

func (a *application) prompt(label, defaultValue string) (string, error) {
	fmt.Fprintf(a.out, "%s [%s]: ", label, defaultValue)
	line, err := a.inputReader().ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func (a *application) confirm(label string, defaultValue bool) (bool, error) {
	hint := "y/N"
	if defaultValue {
		hint = "Y/n"
	}
	fmt.Fprintf(a.out, "%s [%s]: ", label, hint)
	line, err := a.inputReader().ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultValue, nil
	}
	return line == "y" || line == "yes", nil
}

func (a *application) inputReader() *bufio.Reader {
	if a.reader == nil {
		a.reader = bufio.NewReader(a.in)
	}
	return a.reader
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", "_", " ", "_").Replace(value)
	if value == "" {
		return "task"
	}
	return value
}

func stateName(active bool) string {
	if active {
		return "enabled"
	}
	return "disabled"
}
