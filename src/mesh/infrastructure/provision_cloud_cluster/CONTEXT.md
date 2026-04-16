# Feature: Provision Cloud Cluster (AWS)

**Description:**
Composes lower-level provisioning primitives to deploy the standard "Scavenger Mesh" topology on AWS. This acts as the entry point for the Pulumi stack.

## 🧩 Interface

| Type | Name | Data Type | Description |
| :--- | :--- | :--- | :--- |
| **Input** | `Pulumi Config` | `Map` | Standard Pulumi configuration (Region, etc.). |
| **Input** | `Secrets` | `Env` | `TAILSCALE_KEY` loaded from environment. |
| **Output** | `leader_public_ip` | `string` | Public IP of the Leader node. |
| **Output** | `worker_public_ip` | `string` | Public IP of the Worker node. |

## 🏗 Topology
- **1x Leader Node**: `t3.small` (Server Role: Nomad/Consul Server, Traefik). Bootstraps the cluster.
- **1x Worker Node**: `t3.micro` (Client Role: Nomad/Consul Client). Joins the Leader via MagicDNS.
