"""
Feature: Deploy Traefik with TLS/HTTPS
Deploys Traefik ingress controller with Let's Encrypt ACME.
"""

from .deploy import deploy_traefik

__all__ = [
    "deploy_traefik",
]
