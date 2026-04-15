# Feature: Configure Tailscale

**Description:**
Manages the integration with Tailscale, specifically generating authentication keys required for nodes to join the mesh.

## 🧩 Interface

| Type | Name | Data Type | Description |
| :--- | :--- | :--- | :--- |
| **Input** | `key_name` | `string` | The name to assign to the generated key. |
| **Input** | `ephemeral` | `boolean` | Whether the key (and nodes using it) are ephemeral. Default: True. |
| **Input** | `reusable` | `boolean` | Whether the key can be used multiple times. Default: True. |
| **Input** | `tags` | `list[string]` | ACL tags to apply to the key. Default: ["tag:mesh"]. |
| **Output** | `auth_key` | `string` | The generated Tailscale authentication key (Secret). |

## 🧪 Tests
- [ ] Test_CreateKey_DefaultValues_Success: Verify key creation with default parameters (ephemeral=True, reusable=True, tag="tag:mesh").
- [ ] Test_CreateKey_CustomTags_Success: Verify key creation with custom tags provided as input.
- [ ] Test_CreateKey_SecretOutput: Verify the returned auth_key is wrapped as a Pulumi Secret.
- [ ] Test_CreateKey_NonEphemeral_Success: Verify key creation when ephemeral is set to False.
