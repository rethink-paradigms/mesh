from mesh.infrastructure.config.env import get_nomad_addr as _get_nomad_addr_from_env


def get_nomad_addr() -> str:
    return _get_nomad_addr_from_env()
