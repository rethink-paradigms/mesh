# Feature: Provision Local Cluster (Multipass)

**Description:**
Orchestrates the creation and management of a local development cluster using Multipass VMs. Replicates the cloud topology locally.

## 🧩 Interface

| Type | Name | Data Type | Description |
| :--- | :--- | :--- | :--- |
| **Command** | `up` | `CLI` | Provisions Leader and Worker VMs, and syncs secrets. |
| **Command** | `down` | `CLI` | Destroys all local VMs. |
| **Command** | `status` | `CLI` | Lists running VMs. |
| **Command** | `provision` | `CLI` | Re-runs the boot script on a specific node (idempotent update). |

## 🏗 Topology
- **local-leader**: `2CPU, 1GB`.
- **local-worker**: `1CPU, 512MB`.
