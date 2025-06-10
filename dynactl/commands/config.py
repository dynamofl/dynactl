"""
Implementation of the 'dynactl config' command
"""

import click
import logging
from typing import Optional

from ..utils.config_manager import ConfigManager
from ..cli import pass_global_options, GlobalOptions

logger = logging.getLogger("dynactl")


@click.group(name="config")
def config_group():
    """Get and set configuration for dynactl.
    
    Configuration is saved into the .dynactl/config file.
    """
    pass


@config_group.command(name="get")
@click.argument("key", required=True)
@pass_global_options
def config_get(global_options: GlobalOptions, key: str):
    """Fetch the value of a configuration key.
    
    Returns 'Invalid key' error message if the key is not supported.
    Returns 'The value is unset' error message upon error.
    """
    config_manager = ConfigManager(global_options.config_file)
    
    value = config_manager.get(key)
    if value is None:
        if key in ConfigManager.VALID_KEYS:
            click.echo(f"$ [{key}]: <unset>")
        else:
            click.echo(f"Invalid key: {key}")
            return 1
    else:
        click.echo(f"$ [{key}]: {value}")
    return 0


@config_group.command(name="set")
@click.argument("key", required=True)
@click.argument("value", required=True)
@pass_global_options
def config_set(global_options: GlobalOptions, key: str, value: str):
    """Set the value of a configuration key.
    
    Returns 'Invalid key' error message if the key is not supported.
    Returns 'Invalid value' error message if the value is not supported.
    """
    config_manager = ConfigManager(global_options.config_file)
    
    if config_manager.set(key, value):
        click.echo(f"$ Updated property [{key}]: {value}")
        return 0
    else:
        return 1


@config_group.command(name="list")
@pass_global_options
def config_list(global_options: GlobalOptions):
    """List all configuration settings."""
    config_manager = ConfigManager(global_options.config_file)
    config_data = config_manager.get_all()
    
    if not config_data:
        click.echo("No configuration settings found.")
        return 0
        
    click.echo("Current configuration:")
    for key, value in sorted(config_data.items()):
        # Mask sensitive values
        if key.endswith(".password") or key.endswith(".token"):
            value = "********"
        click.echo(f"[{key}]: {value}")
    
    return 0


@config_group.command(name="unset")
@click.argument("key", required=True)
@pass_global_options
def config_unset(global_options: GlobalOptions, key: str):
    """Remove a configuration key."""
    config_manager = ConfigManager(global_options.config_file)
    
    if config_manager.unset(key):
        click.echo(f"$ Property [{key}] unset")
        return 0
    else:
        return 1 