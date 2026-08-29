# RunBinder

RunBinder keeps scheduled commands next to the code they operate on.

Each task is a small YAML file that describes what to run, where to run it, and
when it is due. You register that file once, then a single local service runs
enabled tasks and records their results. Definitions remain portable and
reviewable; runtime state stays local.

RunBinder is one Go binary backed by SQLite. It does not require a language
runtime, external database, or system cron configuration.

## How it works

```text
task.runbinder.yaml  -- add/update -->  local registry  <-- service -->  commands
                                             |
                                      history and status
```

The registry stores a validated snapshot of each definition. This is
intentional: editing a YAML file does not change a running schedule until you
apply it with `runbinder update`.

## Install

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

## Task definitions

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

# Runs of one task do not overlap unless explicitly allowed.
allow_overlap: false
```

All schedules use the service's local time zone. Clock values must use
`HH:MM:SS`. When multiple schedule forms produce the same instant, RunBinder
runs the task once.

Commands are executed through the platform shell in `working_dir`. Standard
output and standard error are appended to `.runbinder.log` in that directory.
Treat task definitions like shell scripts: only register files you trust.

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

Different tasks may run concurrently up to the configured limit. A task does
not overlap itself unless `allow_overlap` is true. If the service is delayed or
the machine wakes from sleep, occurrences older than `--misfire-grace` are
skipped instead of creating an unbounded catch-up queue.

Only one service can use a RunBinder storage directory at a time. Detached mode
persists the service PID, sends `SIGTERM` for graceful shutdown, and writes
service output to RunBinder's internal log. It does not automatically restart
after a machine reboot or process crash.

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
| `runbinder nuke` | Delete all registrations and run history |

Run `runbinder COMMAND --help` for flags and command-specific usage.

## Local state

RunBinder stores its database, service lock, PID file, and internal log under:

```text
~/.local/share/runbinder/
```

Set `RUNBINDER_HOME` to use another location. `runbinder status` prints the
active location. `runbinder nuke` removes the database and history but leaves
task definitions and task-output logs untouched.

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
