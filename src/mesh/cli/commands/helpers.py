"""
Shared helper functions for CLI commands.
"""

import os


def get_nomad_addr() -> str:
    return os.environ.get("NOMAD_ADDR", "") or "http://127.0.0.1:4646"
