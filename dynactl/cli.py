"""
Main CLI entry point for dynactl command processing
"""

import os
import sys
import click
import logging
from typing import Optional

from . import __version__


# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(levelname)s: %(message)s"
)
logger = logging.getLogger("dynactl")


# Common options for all commands
class GlobalOptions:
    def __init__(self, verbose: int, config_file: Optional[str] = None):
        self.verbose = verbose
        self.config_file = config_file or os.path.expanduser("~/.dynactl/config")
        
        # Set verbosity level
        if verbose == 0:
            logger.setLevel(logging.WARNING)
        elif verbose == 1:
            logger.setLevel(logging.INFO)
        elif verbose >= 2:
            logger.setLevel(logging.DEBUG)


# Pass context between commands
pass_global_options = click.make_pass_decorator(GlobalOptions)


# Main CLI group
@click.group()
@click.option("--verbose", "-v", count=True, help="Increase verbosity (can be used multiple times)")
@click.option("--config-file", help="Path to config file", default=None)
@click.version_option(__version__, prog_name="dynactl")
@click.pass_context
def cli(ctx, verbose, config_file):
    """Dynamo AI Deployment Tool.
    
    A Python based tool to manage customer's DevOps operations
    on Dynamo AI deployment and maintenance.
    """
    # Create a GlobalOptions object and store it in the context
    ctx.obj = GlobalOptions(verbose=verbose, config_file=config_file)


# Import command modules
from .commands.config import config_group
from .commands.artifacts import artifacts_group
from .commands.cluster import cluster_group
from .commands.validate import validate_command

# Register command groups
cli.add_command(config_group)
cli.add_command(artifacts_group)
cli.add_command(cluster_group)
cli.add_command(validate_command)


def main():
    """Main entry point for the CLI"""
    try:
        # Support for PyInstaller bundled application
        # PyInstaller creates a temp folder and stores path in _MEIPASS
        if getattr(sys, 'frozen', False):
            # If the application is run as a bundle, the PyInstaller bootloader
            # extends the sys module by a flag frozen=True and sets the app 
            # path into variable _MEIPASS
            application_path = sys._MEIPASS
            os.environ['DYNACTL_BUNDLED'] = 'true'
            logger.debug(f"Running as PyInstaller bundle from {application_path}")
        else:
            application_path = os.path.dirname(os.path.abspath(__file__))
            logger.debug(f"Running from source at {application_path}")
        cli()
    except Exception as e:
        logger.error(f"Error: {str(e)}")
        if logger.level == logging.DEBUG:
            # Show full traceback in debug mode
            raise
        sys.exit(1)


if __name__ == "__main__":
    main() 