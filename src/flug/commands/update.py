import click
import yaml
import os
from pony.orm import db_session
from flug.utils.db_actions import Tasks, assert_db_initialized
import hashlib
from pathlib import Path


@click.command()
@click.argument("file_path", type=click.Path(exists=True, dir_okay=False))
def update(file_path):
    """
    Update a registered task with changes from its YAML file.
    
    Flug does not automatically detect changes to task definition files. Use this
    command after modifying a task's YAML file to apply the changes.
    
    \b
    Example:
        $ flug update my_task.flug.yaml
    """
    assert_db_initialized()
    _internal(file_path)


@db_session
def _internal(file_path):
    abs_path = Path(os.path.abspath(file_path))
    working_dir = str(abs_path.parent)
    with open(abs_path, "r") as f:
        data = yaml.safe_load(f)

    namespace = data.get("namespace")
    working_dir_from_yaml = data.get("working_dir")

    if working_dir_from_yaml is not None:
        working_dir = working_dir_from_yaml

    registered_task = Tasks.get(namespace=namespace)

    if registered_task is None:
        print("[FLUG] Cannot update a task that has not yet been registered")
        return

    yaml_string = yaml.dump(data)
    yaml_hash = hashlib.md5(yaml_string.encode("utf-8")).hexdigest()
    has_changes = yaml_hash != registered_task.md5

    if not has_changes:
        print(f"[FLUG] Cannot update because there are no changes")
        return

    registered_task.definition = yaml_string
    registered_task.md5 = yaml_hash
    registered_task.working_dir = working_dir
    print(f"[FLUG] Task has been updated!")
