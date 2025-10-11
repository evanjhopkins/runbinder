import click
from flug.utils.db_actions import Tasks, Run, assert_db_initialized
from pony.orm import db_session, desc
from rich.console import Console
from rich.table import Table as RichTable
from rich.style import Style


@click.command()
def list():
    """
    Display all registered Flug tasks.
    
    Shows a table with task ID, namespace, active status, working directory,
    and last run time with success/failure status.
    
    \b
    Example:
        $ flug list
    """
    assert_db_initialized()
    _internal()

@db_session
def _internal():
    tasks = Tasks.select()[:]

    if tasks is None or len(tasks) == 0:
        print("[FLUG] No tasks have been registered")
        return

    console = Console()
    table = RichTable(show_header=True, header_style="bold", box=None)
    table.add_column("ID")
    table.add_column("Namespace")
    table.add_column("Active")
    table.add_column("Dir")
    table.add_column("Last Run")

    for t in tasks:
        last_run = Run \
            .select(namespace=t.namespace) \
            .order_by(lambda r: desc(r.execution_time)) \
            .first()
        last_run_time = "(none)"
        if last_run is not None:
            last_run_time = last_run.execution_time.strftime("%Y-%m-%d %H:%M:%S")       
            post_fix = " (SUCC)" if last_run.status else " (FAIL)"
            last_run_time += post_fix

        
        style = Style(color="green") if t.active else Style(color="grey50")
        table.add_row(
            str(t.id),
            t.namespace,
            str(t.active),
            t.working_dir or "",
            last_run_time,
            style=style
        )

    console.print(table)