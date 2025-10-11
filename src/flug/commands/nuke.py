import click
from flug.utils.db_actions import nuke_db

@click.command()
def nuke():
    """
    Delete the Flug database and all task registrations.
    
    WARNING: This permanently removes all registered tasks, run history, and
    service data. This action cannot be undone. Task YAML files are not deleted.
    
    \b
    Example:
        $ flug nuke
    """
    nuke_db()