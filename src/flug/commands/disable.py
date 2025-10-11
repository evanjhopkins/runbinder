import click
from pony.orm import db_session
from flug.utils.db_actions import assert_db_initialized
from flug.utils.messaging import FAILED_TO_RESOLVE_TASK
from flug.utils.resolve_task import resolve_task


@click.command()
@click.argument("target", type=str)
def disable(target):
    """
    Disable a task to prevent it from running.
    
    The task remains registered but will not execute on its schedule until re-enabled.
    TARGET can be either a task namespace or a path to the task YAML file.
    
    \b
    Examples:
        $ flug disable my_task.flug.yaml
        $ flug disable production.myproject.task1
    """
    assert_db_initialized()
    _internal(target)

@db_session
def _internal(target):
    task = resolve_task(target)
    if task is None:
        print(FAILED_TO_RESOLVE_TASK)
        return

    if not task.active:
        print(f"[FLUG] Task '{task.namespace}' is already stopped.")
        return

    task.active = False
    print(f"[FLUG] Task '{task.namespace}' has been marked as stopped.")
