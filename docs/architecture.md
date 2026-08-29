# Architecture

RunBinder is organized around application use cases and independent technical
boundaries rather than around CLI commands. Dependencies point inward toward
typed domain values:

```text
CLI commands --> application use cases
                       |
                       +--> task configuration --> scheduler planner
                       |
                       +--> repository interface --> SQLite
                       |
                       +--> process runner --> shell + task log
```

## Boundaries

`internal/cli` is a thin delivery adapter. Its files construct Cobra commands,
validate command-line-only concerns, prompt for input, handle terminal signals,
and render results. It does not open the database or coordinate service
processes.

`internal/app` owns RunBinder's use cases. It resolves task references, controls
repository lifetimes, executes immediate runs, and manages the foreground or
detached scheduling service. Keeping these operations independent of Cobra
makes them reusable and testable as complete workflows.

`internal/taskconfig` owns the public YAML contract. It performs strict parsing,
validation, canonical serialization, and hashing before a definition reaches
storage. This makes invalid tasks a CLI error instead of a service-time surprise.

`internal/store` is the state boundary. The rest of the application uses its
repository interface and does not know SQL. SQLite runs in WAL mode with a busy
timeout because the service and short-lived CLI processes access it together.
Numbered, transactional migrations are recorded in `schema_migrations` at
startup. Unversioned pre-RunBinder databases are intentionally rejected rather
than carrying compatibility code; this project is still pre-v1 and local state
can be reset with `runbinder nuke --yes`.

`internal/scheduler` is a pure planner. Given a task and a time range, it returns
the due executions in `(after, through]`. It does no sleeping, database work, or
process execution, which makes time boundaries and daylight-saving behavior
testable without running a daemon.

`internal/runner` is the side-effect boundary. It executes through the platform
shell, appends stdout and stderr to the task's log, and records start, finish,
scheduled time, result, and error text.

`internal/service` coordinates these parts. A one-second ticker advances the
scheduler cursor. Jobs run asynchronously behind a bounded semaphore, while a
per-namespace reservation prevents accidental overlap. The service lock allows
only one Unix service process per RunBinder storage directory. Detached mode
re-executes the binary in a new session, publishes its PID, and confirms a fresh
heartbeat before the parent exits.

## Scheduling policies

RunBinder favors predictable behavior over unbounded catch-up:

- Disabled tasks never enter the planner.
- The start of each tick range is exclusive and the end is inclusive.
- Duplicate occurrences from combined schedule forms are executed once.
- A delayed occurrence older than `--misfire-grace` is skipped.
- Runs of the same task do not overlap unless `allow_overlap` is true.
- Different tasks share a configurable global concurrency limit.
- On shutdown, new work stops, running child processes receive cancellation,
  and the service waits for runner bookkeeping to finish.

These policies prevent the two common scheduler failure cascades: thousands of
stale jobs after downtime and a slow task spawning unlimited copies of itself.

## Reliability choices

The Go implementation makes several behaviors explicit:

- YAML command lists are joined with `&&`; they are no longer passed as an
  invalid subprocess argument.
- Only enabled tasks execute.
- The service sleeps on a ticker instead of busy-spinning.
- Task execution does not block schedule evaluation for every other task.
- `remove` deletes the resolved task rather than the input string.
- Invalid cron expressions, times, intervals, and unknown fields are rejected
  during registration or update.
- CLI failures return non-zero exit codes.

Canonical task-definition hashes use SHA-256, and CLI failures return non-zero
exit codes so RunBinder composes cleanly with scripts and service managers.
