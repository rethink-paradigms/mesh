# Reusable Workflows

## Micro-Context
This directory defines the "CI/CD Logic" as a product. These are **Reusable Workflows** designed to be called by other repositories. They abstract away the complexity of building Docker images, connecting to the Mesh, and talking to Nomad.

## External Consumption
*   **Called By**: Application repository `.github/workflows/*.yml` files.
*   **Syntax**: `uses: rethink-paradigms/infa/.github/workflows/deploy-paas.yml@main`.
*   **Role**: The bridge between Source Code (Git) and Runtime (Nomad).
