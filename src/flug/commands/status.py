import click
from flug.utils.db_actions import HeartBeat, assert_db_initialized
from pony.orm import db_session
from datetime import datetime
from flug.utils.general import get_storage_dir
from flug.utils.logging import print_internal_log
from rich import print


@click.command()
def status():
    """
    Check the status of the Flug service.
    
    Displays whether the Flug service is running, the last heartbeat time,
    internal storage location, and recent service logs.
    
    \b
    Example:
        $ flug status
    """
    assert_db_initialized()   
    _internal()


@db_session
def _internal():
    colored_is_running = "NO"
    last_hb = "(none)"
    storage_dir = get_storage_dir()

    hb = HeartBeat.get(name="service")
    if hb is not None:
        elapsed_sec = (datetime.now() - hb.last).total_seconds()
        color = "green" if elapsed_sec < 5 else "red"
        is_running = "YES" if elapsed_sec < 5 else "NO"
        colored_is_running = f"[{color}]{is_running}[/{color}]"
        last_hb = hb.last.strftime("%Y-%m-%d %H:%M:%S")
    print("Is Service Running:", colored_is_running)
    print("Last Heartbeat:", last_hb)
    print("Internal Storage:", storage_dir)
    print("Recent Logs:")
    print_internal_log(n=5, prefix="-> ")