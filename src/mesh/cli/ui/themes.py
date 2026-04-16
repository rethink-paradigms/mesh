"""
Theme constants for the mesh CLI.
Consistent colors, styles, and branding across all CLI output.
"""

from rich.theme import Theme
from rich.style import Style

# Brand colors
MESH_CYAN = "#00d4ff"
MESH_GREEN = "#00ff88"
MESH_YELLOW = "#ffd700"
MESH_RED = "#ff4444"
MESH_PURPLE = "#b388ff"
MESH_DIM = "#666666"
MESH_WHITE = "#e0e0e0"
MESH_ORANGE = "#ff9800"

# Semantic styles
STYLE_SUCCESS = Style(color=MESH_GREEN, bold=True)
STYLE_ERROR = Style(color=MESH_RED, bold=True)
STYLE_WARNING = Style(color=MESH_YELLOW, bold=True)
STYLE_INFO = Style(color=MESH_CYAN)
STYLE_DIM = Style(color=MESH_DIM)
STYLE_APP = Style(color=MESH_PURPLE, bold=True)
STYLE_NODE = Style(color=MESH_ORANGE, bold=True)
STYLE_HEADER = Style(color=MESH_CYAN, bold=True)

# Rich theme for Console
MESH_THEME = Theme({
    "info": "bold #00d4ff",
    "success": "bold #00ff88",
    "warning": "bold #ffd700",
    "error": "bold #ff4444",
    "app": "bold #b388ff",
    "node": "bold #ff9800",
    "dim": "#666666",
    "header": "bold #00d4ff",
    "mesh.brand": "bold #00d4ff",
    "mesh.accent": "#b388ff",
})

# ASCII art banner
BANNER = """[bold #00d4ff]
 ███╗   ███╗███████╗███████╗██╗  ██╗
 ████╗ ████║██╔════╝██╔════╝██║  ██║
 ██╔████╔██║█████╗  ███████╗███████║
 ██║╚██╔╝██║██╔══╝  ╚════██║██╔══██║
 ██║ ╚═╝ ██║███████╗███████║██║  ██║
 ╚═╝     ╚═╝╚══════╝╚══════╝╚═╝  ╚═╝[/]
[#666666]  Infrastructure Platform[/]
[#666666]  Deploy anywhere. From a Pi to a thousand nodes.[/]
"""

# Status indicators
STATUS_ICONS = {
    "running": "🟢",
    "stopped": "🔴",
    "starting": "🟡",
    "error": "❌",
    "healthy": "💚",
    "unhealthy": "💔",
    "leader": "👑",
    "worker": "⚙️",
    "app": "📦",
    "mesh": "🔗",
}
