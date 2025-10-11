import click
from pony.orm import db_session
from flug.utils.constants import LOG_FILE_POSTFIX
from flug.utils.db_actions import assert_db_initialized
from flug.utils.messaging import FAILED_TO_RESOLVE_TASK
from flug.utils.resolve_task import resolve_task

TAIL = 20

@click.command()
@click.argument("target", type=str)
def log(target):
    """
    View the execution logs for a task.
    
    Shows the last 20 lines of output from task executions. Logs include both
    stdout and stderr from the task's commands.
    TARGET can be either a task namespace or a path to the task YAML file.
    
    \b
    Examples:
        $ flug log my_task.flug.yaml
        $ flug log production.myproject.task1
    """
    assert_db_initialized()
    _internal(target)

@db_session
def _internal(target):
    task = resolve_task(target)
    if task is None:
        print(FAILED_TO_RESOLVE_TASK)
        return

    log_file_path = task.working_dir + "/" + LOG_FILE_POSTFIX
    try:
        with open(log_file_path, "r", encoding="utf-8") as f:
            lines = f.readlines()
        for line in lines[-TAIL:]:
            print(line, end="")
    except FileNotFoundError:
        print("(none)")
