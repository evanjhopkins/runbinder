import click
from flug.utils.db_actions import assert_db_initialized
from pony.orm import db_session
from flug.utils.messaging import FAILED_TO_RESOLVE_TASK
from flug.utils.resolve_task import resolve_task


@click.command()
@click.argument("target", type=str)
def remove(target):
    """
    Completely remove a task from Flug.
    
    This permanently deletes the task registration. The YAML file is not deleted.
    TARGET can be either a task namespace or a path to the task YAML file.
    
    \b
    Examples:
        $ flug remove my_task.flug.yaml
        $ flug remove production.myproject.task1
    """
    assert_db_initialized()
    _internal(target)

@db_session
def _internal(target):
    task = resolve_task(target)
    if task is None:
        print(FAILED_TO_RESOLVE_TASK)
        return

    target.delete()
    print("[FLUG] removed task:", target.namespace)
