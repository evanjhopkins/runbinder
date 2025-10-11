import click
from flug.utils.constants import LOG_FILE_POSTFIX
from flug.utils.db_actions import Tasks, HeartBeat, Run, assert_db_initialized
from pony.orm import db_session, commit, rollback
import yaml
from datetime import datetime, timedelta
from dataclasses import dataclass
import subprocess
import time
from croniter import croniter
from flug.utils.logging import log_internal
import hashlib
import concurrent.futures


@dataclass
class ScheduledExecution:
    namespace: str
    execute_at: datetime
    cmd: str
    working_dir: str


def time_str_to_dt(time_str: str, date_o):
    time_o = datetime.strptime(time_str, "%H:%M:%S").time()
    return datetime.combine(date_o, time_o)


@db_session
def update_heartbeat(name: str):
    now = datetime.now()
    hb = HeartBeat.get(name=name)
    if hb:
        hb.last = now
    else:
        HeartBeat(name=name, last=now)
    commit()

def build_ex_command(raw_cmd: list[str]):
    cmd = (
        " && ".join(raw_cmd) if isinstance(raw_cmd, tuple) else raw_cmd
    )
    return cmd


@db_session
def get_enabled_tasks_hash():
    hashes = [t.md5 for t in list(Tasks.select(lambda t: t.active))]
    return hashlib.md5("".join(hashes).encode("utf-8")).hexdigest()

@db_session
def log_run(namespace: str, execution_time: datetime, status: bool):
    Run(namespace=namespace,execution_time=execution_time, status=status)
    commit()

@db_session
def get_scheduled_executions(now=datetime.now()):
    run_date = now.date()
    tasks = Tasks.select()[:]
    executions = []

    for task in tasks:
        config = yaml.safe_load(task.definition)
        ex_times = set()
        cron = config.get("cron", None)

        if cron is not None:
            end = now + timedelta(days=1)
            it = croniter(cron, now)
            next_time = it.get_next(datetime)
            while next_time < end:
                ex_times.add(next_time)
                next_time = it.get_next(datetime)

        schedule = config.get("schedule", None)
        if schedule is not None:
            # handle simple time of day
            time_of_day = schedule.get("time_of_day", None)
            if time_of_day is not None:
                for time_str in time_of_day:
                    execute_at = time_str_to_dt(time_str, run_date)
                    if execute_at > now:
                        ex_times.add(execute_at)
                   
            # handle start & end with interval
            window_interval = schedule.get("window_interval", None)
            if window_interval is not None:
                first_str = window_interval.get("start", None)
                last_str = window_interval.get("stop", None)
                interval_sec = window_interval.get("interval_sec", None)
                if any(x is None for x in (first_str, last_str, interval_sec)):
                    print(
                        f"[FLUG] Unable to use window_interval schedule for {task.namespace}, skipping."
                    )
                    continue

                first_dt = time_str_to_dt(first_str, run_date)
                last_dt = time_str_to_dt(last_str, run_date)
                step = timedelta(seconds=interval_sec)
                curr = first_dt
                while curr <= last_dt:
                    ex_dt = curr
                    if ex_dt > now:
                        ex_times.add(ex_dt)    
                    curr += step

        
        raw_cmd = config.get("command", None)
        cmd = build_ex_command(raw_cmd)
        for ex_time in ex_times:
            ex = ScheduledExecution(
                namespace=task.namespace,
                execute_at=ex_time,
                cmd=cmd,
                working_dir=task.working_dir,
            )
            executions.append(ex)
    # sort to be in time order
    sorted_executions = sorted(executions, key=lambda x: x.execute_at)

    return sorted_executions


@click.command()
def service():
    """
    Start the Flug service to execute scheduled tasks.
    
    This is a long-running process that monitors registered tasks and executes
    them according to their schedules. The service should typically be run as a
    background process or system service.
    
    \b
    Examples:
        $ flug service
        $ nohup flug service > /dev/null 2>&1 &
    """
    assert_db_initialized()
    log_internal("FLUG Service started", print_in_console=True)
    scheduled_executions = get_scheduled_executions()
    hash = get_enabled_tasks_hash()
    curr_date = datetime.now().date()

    def run_task(ex):
        start_time = datetime.now()
        try:
            log_file = ex.working_dir + "/" + LOG_FILE_POSTFIX
            with open(log_file, "a", encoding="utf-8") as log:
                subprocess.run(
                    ex.cmd,
                    cwd=ex.working_dir,
                    shell=True,
                    stdout=log,
                    stderr=log,
                    text=True,
                    check=True,
                )
            log_run(ex.namespace, start_time, 1)
        except subprocess.CalledProcessError:
            log_internal(f"{ex.namespace} FAILED", print_in_console=True)
            log_run(ex.namespace, start_time, 0)

    try:
        with concurrent.futures.ThreadPoolExecutor() as executor:
            while True:
                update_heartbeat("service")
                tick_start_time = datetime.now()
                to_execute = [
                    e for e in scheduled_executions if e.execute_at <= tick_start_time
                ]
                scheduled_executions = [
                    e for e in scheduled_executions if e.execute_at > tick_start_time
                ]
                if to_execute:
                    futures = [executor.submit(run_task, ex) for ex in to_execute]
                    concurrent.futures.wait(futures)

                curr_hash = get_enabled_tasks_hash()    
                have_configs_changes = curr_hash != hash
                if have_configs_changes:
                    hash = curr_hash

                if tick_start_time.date() > curr_date or have_configs_changes:
                    scheduled_executions = get_scheduled_executions(tick_start_time)
                    curr_date = tick_start_time.date()
                    print(f"Rebuilding schedules {str(tick_start_time)}")

    except KeyboardInterrupt:
        log_internal("FLUG service stopped", print_in_console=True)
        try:
            rollback()
        except Exception:
            pass
