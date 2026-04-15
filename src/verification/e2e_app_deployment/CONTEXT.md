# Feature: E2E App Deployment Verification

**Description:**
Verifies the complete flow of deploying a web application to the running cluster and accessing it via the Ingress controller. This is a "Black Box" system test.

## 🧩 Interface

| Type | Name | Data Type | Description |
| :--- | :--- | :--- | :--- |
| **Input** | `target_env` | `string` | "local" (multipass) or "aws" (pulumi stack). |
| **Input** | `leader_ip` | `string` | The public IP of the Leader/Ingress node. |
| **Output** | `status` | `string` | "Pass" if application is reachable and content matches. |

## 🧪 Scenarios
- [ ] **Deploy & Reach**: Deploy the marketing site and verify `HTTP 200` and content "Hello World" via Traefik.
- [ ] **Ingress Routing**: Verify that the `Host` header correctly routes traffic to the specific service.
