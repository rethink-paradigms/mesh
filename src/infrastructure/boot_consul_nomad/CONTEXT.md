# Feature: Boot Consul & Nomad

**Description:**
Handles the bootstrap process for a node, installing dependencies, joining the mesh, and starting the cluster services.

## 🧩 Interface

| Type | Name | Data Type | Description |
| :--- | :--- | :--- | :--- |
| **Input** | `tailscale_key` | `string` | Auth key for Tailscale. |
| **Input** | `leader_ip` | `string` | IP/Hostname of the cluster leader. |
| **Input** | `role` | `string` | "server" or "client". |
| **Input** | `cluster_tier` | `string` | Cluster tier: "lite", "standard", "ingress", "production" (default: "production"). |
| **Input** | `enable_caddy` | `string` | "true" or "false" - auto-derived from cluster_tier (lite/standard = true). |
| **Output** | `boot_script_sh` | `string` | The rendered shell script for direct execution (e.g., AWS User Data). |
| **Output** | `boot_script_cloud_init_yaml` | `string` | The rendered cloud-init YAML for system initialization (e.g., Multipass). |
| **Output** | `status` | `string` | "Healthy" if services are running. |

## 🧪 Tests
- [ ] Test_BootScript_Rendering_Shell: Verify boot.sh template renders with correct variables (TAILSCALE_KEY, LEADER_IP, ROLE).
- [ ] Test_BootScript_Rendering_CloudInit: Verify cloud-init YAML is correctly generated and includes the shell script.
- [ ] Test_BootScript_Syntax: Verify all shell scripts have valid shebangs and basic syntax.
- [ ] Test_BootScript_Idempotency: Verify scripts check for existing binaries/configs before installing (manual verification logic).
- [ ] Test_BootScript_Permissions: Verify all scripts are executable.
