# RunBinder

RunBinder keeps scheduled commands next to the code they operate on.

Each task is a small YAML file that describes what to run, where to run it, and
when it is due. You register that file once, then a single local service runs
enabled tasks and records their results. Definitions remain portable and
reviewable; runtime state stays local.

RunBinder is one Go binary backed by SQLite. It does not require a language
runtime, external database, or system cron configuration.

## Features

- Version-controlled YAML definitions with strict validation.
- Cron expressions, fixed times of day, and fixed intervals within a daily
  window; schedule forms can be combined.
- Per-task IANA time zones and optional overlapping execution.
- Foreground or detached scheduling with a configurable global concurrency
  limit and misfire grace period.
- Immediate execution, enable/disable controls, definition-drift reporting,
  run history, task output, and service health from one CLI.
- Local SQLite state with no external service dependency.

## How it works

```text
task.runbinder.yaml  -- add/update -->  local registry  <-- service -->  commands
                                             |
                                      history and status
```

The registry stores a validated snapshot of each definition. This is
intentional: editing a YAML file does not change a running schedule until you
apply it with `runbinder update`.

## Requirements and platforms

Building requires Go 1.24 or newer. At runtime, task commands require the tools
they invoke and a writable working directory. RunBinder uses `/bin/sh -c` on
non-Windows systems and `cmd.exe /C` on Windows.

Linux and macOS support the complete service lifecycle, including detached
operation, process stopping, and a per-storage-directory service lock. Windows
can execute tasks and run the scheduler in the foreground, but detached mode,
service stopping, and singleton enforcement are not implemented.

## Installation

Install the latest release with Go 1.24 or newer:

```sh
go install github.com/evanjhopkins/RunBinder/cmd/runbinder@latest
```

To install the current checkout instead:

```sh
go install ./cmd/runbinder
```

Ensure `$(go env GOPATH)/bin`, or your configured `GOBIN`, is on `PATH`.

## Quick start

Create `heartbeat.runbinder.yaml` in a project directory:

```yaml
namespace: example.heartbeat
command: echo "RunBinder fired at $(date)"
schedule:
  window_interval:
    start: "00:00:00"
    stop: "23:59:59"
    interval_sec: 60
```

Register, test, and enable it:

```sh
runbinder add heartbeat.runbinder.yaml
runbinder run example.heartbeat
runbinder enable example.heartbeat
```

Start the scheduler in the background:

```sh
runbinder service --detach
```

RunBinder waits for a healthy heartbeat before returning. The service keeps
running after the terminal closes.

Check the service and task output from another terminal:

```sh
runbinder status
runbinder list
runbinder log example.heartbeat
runbinder service stop
```

You can also run `runbinder init` inside a project to generate and optionally
register a starter definition interactively.

## Configuration

A definition requires a unique `namespace`, a `command`, and at least one
schedule. Unknown fields and invalid schedules are rejected during `add` or
`update`.

```yaml
namespace: reports.daily

# A string runs as-is. A list is joined with && and stops on the first failure.
command:
  - . .venv/bin/activate
  - python generate_report.py

# Five-field cron, six-field cron with seconds, and cron descriptors work.
cron: "0 6 * * 1-5"

# Schedule forms can be combined with cron.
schedule:
  time_of_day:
    - "12:00:00"
    - "18:00:00"
  window_interval:
    start: "09:00:00"
    stop: "17:00:00"
    interval_sec: 1800

# Defaults to the directory containing this file. Relative paths start there.
working_dir: .

# Optional: evaluate this task's schedule in an IANA time zone.
timezone: America/New_York

# Runs of one task do not overlap unless explicitly allowed.
allow_overlap: false
```

Schedules use the service's local time zone unless a task specifies an IANA
`timezone`, such as `America/New_York`. RunBinder includes the IANA database in
its binary, so these definitions work on minimal server images too. Clock values
must use `HH:MM:SS`. At daylight-saving transitions, schedules follow Go's IANA
time-zone rules; avoid ambiguous transition-hour schedules when exact once-only
wall-clock behavior is required. When multiple schedule forms produce the same
instant, RunBinder runs the task once.

Commands are executed through the platform shell in `working_dir`. Standard
output and standard error are appended to `.runbinder.log` in that directory.
Tasks with the same working directory therefore share one output log. Treat
task definitions like shell scripts: only register files you trust.

| Field | Required | Meaning |
| --- | --- | --- |
| `namespace` | Yes | Unique name in the local registry |
| `command` | Yes | Shell command string, or a list joined with `&&` |
| `cron` | One schedule required | Five-field cron, optional-seconds six-field cron, or a cron descriptor |
| `schedule.time_of_day` | One schedule required | List of local `HH:MM:SS` times |
| `schedule.window_interval` | One schedule required | Inclusive daily window with a positive `interval_sec`; overnight windows are not supported |
| `working_dir` | No | Absolute path, or path relative to the definition; defaults to the definition directory |
| `timezone` | No | IANA time zone; defaults to the service's local zone |
| `allow_overlap` | No | Permit overlapping scheduled runs of this namespace; defaults to `false` |

## Task lifecycle

Register a definition. New tasks are disabled unless `--enable` is supplied:

```sh
runbinder add reports.runbinder.yaml
runbinder add --enable reports.runbinder.yaml
```

After editing a registered definition, apply its new snapshot:

```sh
runbinder update reports.runbinder.yaml
```

Commands that accept `TARGET` can use either the namespace or definition path:

```sh
runbinder run reports.daily
runbinder disable reports.runbinder.yaml
runbinder enable reports.daily
runbinder remove reports.daily
```

`remove` deletes the registration, not the YAML file or task-output log. Existing
run history remains in local storage until `runbinder nuke`.

`run` executes the registered snapshot even when the task is disabled. It does
not apply the scheduler's overlap guard, so an immediate run can overlap a
scheduled run.

## Operating the service

Run in the background for normal use:

```sh
runbinder service --detach
runbinder status
runbinder service stop
```

Run in the foreground when developing or diagnosing the service:

```sh
runbinder service \
  --concurrency 4 \
  --misfire-grace 1m
```

Different tasks may run concurrently up to the configured limit. Scheduled runs
of one task do not overlap unless `allow_overlap` is true. If a running service
is delayed or the machine wakes from sleep, occurrences older than
`--misfire-grace` are skipped instead of creating an unbounded catch-up queue.
The cursor is not persisted, so occurrences missed while the service is stopped
are not recovered when it starts again.

On Unix, only one service can use a RunBinder storage directory at a time.
Detached mode persists the service PID, sends `SIGTERM` for graceful shutdown,
and writes service output to RunBinder's internal log. It does not automatically
restart after a machine reboot or process crash.

## Command reference

| Command | Purpose |
| --- | --- |
| `runbinder init` | Create a starter definition interactively |
| `runbinder add [-e] FILE` | Register a definition, optionally enabled |
| `runbinder update FILE` | Apply changes from a registered definition |
| `runbinder run TARGET` | Execute a registered task immediately |
| `runbinder enable TARGET` | Enable scheduled execution |
| `runbinder disable TARGET` | Pause scheduled execution |
| `runbinder remove TARGET` | Remove a registration |
| `runbinder list` | Show tasks, YAML drift state, directory, and latest result |
| `runbinder log [-n 20] TARGET` | Show recent task output |
| `runbinder status` | Show service health and internal logs |
| `runbinder service [-j 4]` | Run the scheduler in the foreground |
| `runbinder service --detach` | Start the scheduler in the background |
| `runbinder service stop` | Stop the running scheduler |
| `runbinder nuke [-y]` | Delete the registry database and run history |

Run `runbinder COMMAND --help` for flags and command-specific usage.

## Local state

RunBinder stores its database, service lock, PID file, and internal log under:

```text
~/.local/share/runbinder/
```

Set `RUNBINDER_HOME` to use another location. `runbinder status` prints the
active location. `runbinder nuke` removes the database and history but leaves
task definitions, task-output logs, and the internal service log untouched.

## Project structure

```text
cmd/runbinder/        executable entry point
internal/cli/         Cobra commands and terminal output
internal/app/         application workflows
internal/taskconfig/  YAML parsing and validation
internal/scheduler/   occurrence planning
internal/service/     scheduler loop and process control
internal/runner/      shell execution and result recording
internal/store/       SQLite repository and migrations
internal/platform/    state and log paths
```

## Development

Install the current checkout as `runbinderd` so it can coexist with a production
`runbinder` installation:

```sh
./scripts/install_dev.sh
```

The script installs to `GOBIN` when set, otherwise to `$(go env GOPATH)/bin`.
That directory must be on `PATH`.

Run the project checks and build directly with Go:

```sh
gofmt -w cmd internal
go vet ./...
go test -race ./...
go build -o bin/runbinder ./cmd/runbinder
```

See [docs/architecture.md](docs/architecture.md) for implementation boundaries
and scheduler design decisions.

## License

[MIT](LICENSE) Copyright (c) 2025 Evan Hopkins.
