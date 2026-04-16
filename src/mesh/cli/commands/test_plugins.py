"""Tests for the CLI plugin discovery mechanism."""

from unittest.mock import MagicMock, patch
import typer
from mesh.cli.plugins import discover_plugins, PLUGIN_GROUP


def test_discover_plugins_returns_empty_when_no_plugins():
    """Plugin discovery returns empty list when nothing is installed."""
    plugins = discover_plugins()
    assert isinstance(plugins, list)


def test_discover_plugins_loads_valid_plugin():
    """Plugin discovery loads a valid entry point."""
    mock_app = typer.Typer()

    def fake_register():
        return (mock_app, "test-cmd")

    mock_ep = MagicMock()
    mock_ep.load.return_value = fake_register

    with patch("importlib.metadata.entry_points") as mock_eps:
        mock_group = MagicMock()
        mock_group.select.return_value = [mock_ep]
        mock_eps.return_value = mock_group

        plugins = discover_plugins()

    assert len(plugins) == 1
    assert plugins[0] == (mock_app, "test-cmd")


def test_discover_plugins_skips_broken_plugin():
    """Plugin discovery silently skips broken plugins."""
    mock_ep = MagicMock()
    mock_ep.load.side_effect = ImportError("broken")

    with patch("importlib.metadata.entry_points") as mock_eps:
        mock_group = MagicMock()
        mock_group.select.return_value = [mock_ep]
        mock_eps.return_value = mock_group

        plugins = discover_plugins()

    assert plugins == []


def test_discover_plugins_skips_non_typer_return():
    """Plugin discovery skips plugins that don't return a Typer app."""

    def bad_register():
        return ("not-a-typer", "test-cmd")

    mock_ep = MagicMock()
    mock_ep.load.return_value = bad_register

    with patch("importlib.metadata.entry_points") as mock_eps:
        mock_group = MagicMock()
        mock_group.select.return_value = [mock_ep]
        mock_eps.return_value = mock_group

        plugins = discover_plugins()

    assert plugins == []


def test_plugin_group_constant():
    """Plugin group constant is correctly defined."""
    assert PLUGIN_GROUP == "mesh.plugins"
