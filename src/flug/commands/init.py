import click
import os
import yaml
import subprocess
from pathlib import Path
from pony.orm import db_session
from flug.utils.db_actions import assert_db_initialized, Tasks


def get_namespace() -> str:
    """Prompt user for a namespace and return it."""
    default_namespace = Path.cwd().name.lower().replace('-', '_').replace(' ', '_')
    
    click.echo("\nLet's create a new Flug task!")
    click.echo("The namespace should be a unique identifier for your task.")
    click.echo(f"Example: myproject.{default_namespace}.task1\n")
    
    while True:
        namespace = click.prompt(
            "Enter a namespace",
            default=f"myproject.{default_namespace}.task1",
            show_default=True
        )
        
        if namespace.strip():
            return namespace.strip()
        click.echo("Error: Namespace cannot be empty. Please try again.")


def register_task(file_path: Path) -> bool:
    """Register the task with flug."""
    try:
        subprocess.run(["flug", "add", str(file_path)], check=True)
        return True
    except subprocess.CalledProcessError as e:
        click.echo(f"Failed to register task: {e}", err=True)
        return False


def enable_task(file_path: Path) -> bool:
    """Enable the task in flug."""
    try:
        subprocess.run(["flug", "enable", str(file_path)], check=True)
        return True
    except subprocess.CalledProcessError as e:
        click.echo(f"Failed to enable task: {e}", err=True)
        return False


@click.command()
def init():
    """
    Initialize a new Flug task in the current directory.
    
    Creates a task definition file named <directory>.flug.yaml with a basic
    configuration. Optionally registers and enables the task immediately.
    
    \b
    Example:
        $ cd /path/to/my_project
        $ flug init
    
    This will create my_project.flug.yaml with a sample task that runs every minute.
    """
    try:
        _internal()
    except Exception as e:
        click.echo(f"Error initializing Flug task: {str(e)}", err=True)
        raise click.Abort()


def _internal():
    """Internal implementation of the init command."""
    # Get the current working directory name for the task name
    cwd = Path.cwd()
    task_name = cwd.name.lower().replace('-', '_').replace(' ', '_')
    output_file = cwd / f"{task_name}.flug.yaml"
    
    if output_file.exists():
        if not click.confirm(f"File {output_file.name} already exists. Overwrite?"):
            click.echo("Operation cancelled.")
            return
    
    # Get namespace from user
    namespace = get_namespace()
    
    # Create the task definition
    task_definition = {
        'namespace': namespace,
        'command': [
            'echo "Running Flug task"',
            'echo "init task"'
        ],
        'schedule': {
            'window_interval': {
                'start': '00:00:00',
                'stop': '23:59:59',
                'interval_sec': 60
            }
        },
        'working_dir': str(cwd)
    }
    
    # Write to file
    with open(output_file, 'w') as f:
        yaml.dump(task_definition, f, default_flow_style=False, sort_keys=False)
    
    click.echo(f"\n✅ Created new Flug task at: {output_file}")
    
    # Ask if user wants to register the task
    if click.confirm("\nWould you like to register this task with Flug?", default=True):
        if register_task(output_file):
            click.echo("✅ Task registered successfully!")
            
            # Only ask about enabling if registration was successful
            if click.confirm("\nWould you like to enable this task?", default=True):
                if enable_task(output_file):
                    click.echo("✅ Task enabled successfully!")
    
    # Show next steps
    click.echo("\nNext steps:")
    click.echo(f"1. Review the generated {output_file.name} file")
    click.echo(f"2. Run 'flug list' to see your registered tasks")