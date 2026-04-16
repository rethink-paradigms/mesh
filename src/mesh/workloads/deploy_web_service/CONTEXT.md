# Feature: Deploy Web Service

**Description:**
Standardizes the deployment of web applications (Docker containers) to the Nomad cluster, handling ingress routing via Traefik and service discovery via Consul.

## 🧩 Interface

| Type | Name | Data Type | Description |
| :--- | :--- | :--- | :--- |
| **Input** | `app_name` | `string` | Unique name of the application. |
| **Input** | `image` | `string` | Docker image repository. |
| **Input** | `image_tag` | `string` | Docker image tag. Default: "latest". |
| **Input** | `count` | `number` | Number of replicas. Default: 1. |
| **Input** | `port` | `number` | Internal container port. Default: 80. |
| **Input** | `host_rule` | `string` | Traefik Host rule (e.g., "app.example.com"). |
| **Input** | `cpu` | `number` | CPU allocation in MHz. Default: 100. |
| **Input** | `memory` | `number` | Memory allocation in MB. Default: 128. |
| **Output** | `service_name` | `string` | The registered Consul service name. |

## 🧪 Tests
- [ ] Test_NomadTemplate_Syntax: Verify the HCL file is syntactically valid (using nomad fmt/validate if available, or static check).
- [ ] Test_NomadTemplate_Variables: Verify all required variables (app_name, image, host_rule) are defined.
- [ ] Test_NomadTemplate_TraefikTags: Verify Traefik tags are correctly interpolated with ${var.host_rule}.
- [ ] Test_NomadTemplate_SecretTemplate: Verify the secret template block is present for injecting variables.
