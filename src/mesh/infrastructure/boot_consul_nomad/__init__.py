"""
Feature: Boot Consul & Nomad - Script Generation

Generates boot scripts and cloud-init configs for node activation
with Jinja2 template validation.
"""

from .generate_boot_scripts import (
    generate_shell_script,
    generate_cloud_init_yaml,
    validate_rendered_template,
    TemplateValidationError,
)

__all__ = [
    "generate_shell_script",
    "generate_cloud_init_yaml",
    "validate_rendered_template",
    "TemplateValidationError",
]
