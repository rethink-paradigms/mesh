"""
Plugin discovery for the mesh CLI.

Scans for installed packages that register 'mesh.plugins' entry points.
Each plugin must expose a callable that returns a tuple of (typer.Typer, str)
where the string is the sub-command name.

Enterprise and third-party packages register plugins via pyproject.toml:

    [project.entry-points."mesh.plugins"]
    gpu = "mesh_enterprise.cli.gpu:register"
"""

import importlib.metadata
from typing import List, Tuple
import typer

PLUGIN_GROUP = "mesh.plugins"


def discover_plugins() -> List[Tuple[typer.Typer, str]]:
    """
    Discover and load CLI plugins from installed packages.

    Each entry point must be a callable that returns:
        (typer.Typer, str) — the sub-app and its command name

    Returns:
        List of (Typer app, command name) tuples.
        Broken or missing plugins are silently skipped.
    """
    plugins = []
    try:
        eps = importlib.metadata.entry_points()
        # Python 3.12+ returns SelectableGroups; 3.9+ may return dict
        if hasattr(eps, "select"):
            plugin_eps = eps.select(group=PLUGIN_GROUP)
        else:
            plugin_eps = eps.get(PLUGIN_GROUP, [])

        for ep in plugin_eps:
            try:
                plugin_factory = ep.load()
                sub_app, name = plugin_factory()
                if isinstance(sub_app, typer.Typer) and isinstance(name, str):
                    plugins.append((sub_app, name))
            except Exception:
                # Silently skip broken plugins
                pass
    except Exception:
        pass

    return plugins
