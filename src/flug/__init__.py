import click
from flug.commands import add, disable, enable, list, remove, update, status, nuke, log, init
from flug.commands import service


@click.group()
def cli():
    """
    Flug - Flugsicherung: A Python-based CLI tool for managing scheduled process execution.
    
    Flug provides more control than cron with file-based task definitions that can be
    colocated with your codebase and version controlled.
    """
    pass


cli.add_command(status.status)
cli.add_command(nuke.nuke)
cli.add_command(add.add)
cli.add_command(list.list)
cli.add_command(enable.enable)
cli.add_command(remove.remove)
cli.add_command(disable.disable)
cli.add_command(update.update)
cli.add_command(service.service)
cli.add_command(log.log)
cli.add_command(init.init)
