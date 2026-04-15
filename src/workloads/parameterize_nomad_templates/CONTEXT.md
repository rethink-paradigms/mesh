# Feature: Parameterize Nomad Job Templates

**Description:**
Remove hardcoded magic strings from Nomad job templates and replace with configurable variables for better flexibility across different deployment environments.

## Current State

**Hardcoded Values Found:**

1. **datacenter = "dc1"** (6 templates):
   - `deploy_web_service/web_service.nomad.hcl`
   - `deploy_gpu_service/gpu_service.nomad.hcl`
   - `deploy_traefik/traefik.nomad.hcl`
   - `deploy_monitoring/monitoring.nomad.hcl`
   - `deploy_backup_service/backup_service.nomad.hcl` (2 instances)

2. **domain = "localhost"** (1 template):
   - `e2e_multi_node_scenarios/test-web-service.nomad.hcl`

**Issues:**
- ❌ Cannot deploy to multiple datacenters without editing templates
- ❌ Cannot use custom domains without editing templates
- ❌ Templates are not environment-agnostic
- ❌ "dc1" is a default Consul datacenter name (should be explicit)
- ❌ "localhost" only works for local development

## 🧩 Interface

### Enhanced Job Templates

All Nomad job templates will support these new variables:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `datacenter` | string | `"dc1"` | Consul datacenter name |
| `domain` | string | `"localhost"` | Base domain for Traefik routing |

### Updated Template Pattern

**Before (Hardcoded):**
```hcl
job "${var.app_name}" {
  datacenters = ["dc1"]  # ❌ HARDCODED

  group "app" {
    service {
      tags = [
        "traefik.http.routers.${var.app_name}.rule=Host(`${var.app_name}.localhost`)"  # ❌ HARDCODED
      ]
    }
  }
}
```

**After (Parameterized):**
```hcl
variable "datacenter" {
  type    = string
  default = "dc1"
  description = "Consul datacenter name"
}

variable "domain" {
  type    = string
  default = "localhost"
  description = "Base domain for Traefik routing"
}

job "${var.app_name}" {
  datacenters = [var.datacenter]  # ✅ PARAMETERIZED

  group "app" {
    service {
      tags = [
        "traefik.http.routers.${var.app_name}.rule=Host(`${var.app_name}.${var.domain}`)"  # ✅ PARAMETERIZED
      ]
    }
  }
}
```

## 📦 Dependencies

- Nomad (already in use)
- Consul (already in use)
- No new dependencies required

## 🧪 Tests

- [ ] Test: Job deploys with default datacenter value
- [ ] Test: Job deploys with custom datacenter value
- [ ] Test: Job deploys with custom domain value
- [ ] Test: Template validates with all variables provided
- [ ] Test: Backward compatibility (existing jobs still work)
- [ ] Test: Multiple jobs can use different datacenters

## 📝 Design

### Problem Statement

**Why "dc1" is problematic:**
1. **Assumes single datacenter:** Production may have multiple datacenters (dc1, dc2, dc3)
2. **Not explicit:** "dc1" is Consul's default, but should be a conscious choice
3. **Environment conflicts:** Staging and production may need different datacenters

**Why "localhost" is problematic:**
1. **Local-only:** Only works for local development with /etc/hosts
2. **Not production-ready:** Production needs real domains (example.com, app.example.com)
3. **Manual editing:** Must edit template for each environment

### Solution Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Template Parameterization Strategy                          │
│                                                              │
│  1. Add variables block with defaults                        │
│  2. Replace hardcoded values with variable references        │
│  3. Maintain backward compatibility (same defaults)          │
│  4. Enable environment-specific overrides via CLI            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Deployment Scenarios                                        │
│                                                              │
│  Local Development:                                          │
│  nomad job run web.nomad.hcl \                              │
│    -var="datacenter=dc1" -var="domain=localhost"            │
│                                                              │
│  Staging:                                                    │
│  nomad job run web.nomad.hcl \                              │
│    -var="datacenter=dc-staging" \                           │
│    -var="domain=staging.example.com"                        │
│                                                              │
│  Production:                                                 │
│  nomad job run web.nomad.hcl \                              │
│    -var="datacenter=dc-prod" \                              │
│    -var="domain=app.example.com"                            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Implementation Strategy

#### Phase 1: Add Variables to All Templates

For each Nomad job template, add:

```hcl
variable "datacenter" {
  type    = string
  default = "dc1"
  description = "Consul datacenter name"
}

variable "domain" {
  type    = string
  default = "localhost"
  description = "Base domain for Traefik routing (e.g., example.com)"
}
```

#### Phase 2: Replace Hardcoded Values

**datacenter Parameter:**
```hcl
# Before
job "${var.app_name}" {
  datacenters = ["dc1"]
}

# After
job "${var.app_name}" {
  datacenters = [var.datacenter]
}
```

**domain Parameter:**
```hcl
# Before
tags = [
  "traefik.http.routers.${var.app_name}.rule=Host(`${var.app_name}.localhost`)"
]

# After
tags = [
  "traefik.http.routers.${var.app_name}.rule=Host(`${var.app_name}.${var.domain}`)"
]
```

#### Phase 3: Update Documentation

Add variable documentation to each job template:

```hcl
# Run with defaults (local development):
# nomad job run web_service.nomad.hcl \
#   -var="app_name=myapp" \
#   -var="image=nginx" \
#   -var="host_rule=myapp.localhost"
#
# Run with custom datacenter (multi-dc):
# nomad job run web_service.nomad.hcl \
#   -var="app_name=myapp" \
#   -var="image=nginx" \
#   -var="datacenter=dc2" \
#   -var="host_rule=myapp.localhost"
#
# Run with custom domain (production):
# nomad job run web_service.nomad.hcl \
#   -var="app_name=myapp" \
#   -var="image=nginx" \
#   -var="domain=example.com" \
#   -var="host_rule=myapp.example.com"
```

### File-by-File Changes

#### 1. deploy_web_service/web_service.nomad.hcl

**Changes:**
- Add `datacenter` variable
- Add `domain` variable
- Replace `datacenters = ["dc1"]` with `datacenters = [var.datacenter]`
- Update `host_rule` variable description to mention `domain` variable

**Impact:** High - This is the main web service template

#### 2. deploy_gpu_service/gpu_service.nomad.hcl

**Changes:**
- Add `datacenter` variable
- Replace `datacenters = ["dc1"]` with `datacenters = [var.datacenter]`

**Impact:** Medium - GPU-specific template

#### 3. deploy_traefik/traefik.nomad.hcl

**Changes:**
- Add `datacenter` variable
- Replace `datacenters = ["dc1"]` with `datacenters = [var.datacenter]`

**Impact:** High - Traefik is core infrastructure

#### 4. deploy_monitoring/monitoring.nomad.hcl

**Changes:**
- Add `datacenter` variable
- Replace `datacenters = ["dc1"]` with `datacenters = [var.datacenter]`

**Impact:** Medium - Monitoring system job

#### 5. deploy_backup_service/backup_service.nomad.hcl

**Changes:**
- Add `datacenter` variable
- Replace `datacenters = ["dc1"]` with `datacenters = [var.datacenter]` (2 instances)

**Impact:** Medium - Backup service (2 jobs)

#### 6. e2e_multi_node_scenarios/test-web-service.nomad.hcl

**Changes:**
- Add `domain` variable
- Replace `localhost` with variable reference

**Impact:** Low - Test-only template

### Usage Examples

#### Local Development (Defaults)

```bash
# Use default values (dc1, localhost)
nomad job run web_service.nomad.hcl \
  -var="app_name=hello" \
  -var="image=nginx" \
  -var="host_rule=hello.localhost"
```

#### Custom Datacenter

```bash
# Deploy to dc2 instead of dc1
nomad job run web_service.nomad.hcl \
  -var="app_name=hello" \
  -var="image=nginx" \
  -var="datacenter=dc2" \
  -var="host_rule=hello.localhost"
```

#### Custom Domain (Production)

```bash
# Deploy with real domain
nomad job run web_service.nomad.hcl \
  -var="app_name=myapp" \
  -var="image=mycompany/myapp:1.0" \
  -var="domain=example.com" \
  -var="host_rule=myapp.example.com"
```

#### Staging Environment

```bash
# Deploy to staging datacenter with staging domain
nomad job run web_service.nomad.hcl \
  -var="app_name=myapp-staging" \
  -var="image=mycompany/myapp:staging" \
  -var="datacenter=dc-staging" \
  -var="domain=staging.example.com" \
  -var="host_rule=myapp-staging.staging.example.com"
```

### Backward Compatibility

**✅ Fully Backward Compatible:**

1. **Default Values Match Current Behavior:**
   - `datacenter = "dc1"` (same as current hardcoded value)
   - `domain = "localhost"` (same as current hardcoded value)

2. **Existing Commands Still Work:**
   ```bash
   # Before (still works)
   nomad job run web_service.nomad.hcl \
     -var="app_name=hello" \
     -var="image=nginx" \
     -var="host_rule=hello.localhost"

   # After (also works, uses defaults)
   nomad job run web_service.nomad.hcl \
     -var="app_name=hello" \
     -var="image=nginx" \
     -var="host_rule=hello.localhost"
   ```

3. **Optional Overrides:**
   - Only need to specify variables when you want to change defaults
   - Can override datacenter without overriding domain (and vice versa)

### Migration Guide

#### For Existing Deployments

**No Changes Required:**
- Existing jobs continue to work with default values
- `datacenter = "dc1"` and `domain = "localhost"` are defaults

**If You Want to Customize:**

1. **Single Job Update:**
   ```bash
   # Redeploy with custom datacenter
   nomad job run web_service.nomad.hcl \
     -var="app_name=myapp" \
     -var="datacenter=dc2" \
     # ... other vars
   ```

2. **Environment-Specific Config Files:**
   ```bash
   # Create staging.vars
   cat > staging.vars <<EOF
   datacenter = "dc-staging"
   domain = "staging.example.com"
   EOF

   # Use vars file
   nomad job run -var-file=staging.vars web_service.nomad.hcl
   ```

3. **CI/CD Integration:**
   ```bash
   # GitHub Actions example
   nomad job run web_service.nomad.hcl \
     -var="datacenter=${{ env.NOMAD_DATACENTER }}" \
     -var="domain=${{ env.APP_DOMAIN }}"
   ```

## ⚠️ Limitations & Considerations

### Consul Datacenter Configuration

**Note:** Nomad `datacenters` parameter must match Consul datacenter configuration.

**If using custom datacenters:**
1. Configure Consul with datacenter name:
   ```bash
   consul agent -datacenter=dc2 -config-dir=/etc/consul.d/
   ```

2. Configure Nomad to join Consul datacenter:
   ```hcl
   datacenter = "dc2"  # In nomad.hcl
   ```

3. Then Nomad jobs can use:
   ```bash
   nomad job run web.nomad.hcl -var="datacenter=dc2"
   ```

### Domain Name Requirements

**For Custom Domains:**

1. **DNS Configuration:**
   - Domain must resolve to Traefik IP address
   - Can use wildcard DNS: `*.example.com` → Traefik IP

2. **TLS Certificates:**
   - Let's Encrypt requires public DNS
   - For private domains, use self-signed certs or internal CA

3. **Traefik Configuration:**
   - Ensure Traefik listens on correct interface
   - Configure entrypoints for custom domains if needed

### Multi-Datacenter Deployments

**Considerations:**
- **Consul WAN Gossip:** Required for cross-dc communication
- **Nomad Multi-Region:** Advanced setup (separate Nomad regions)
- **Service Discovery:** Services use `datacenter` tag for filtering

## 🔒 Best Practices

### DO ✅

1. **Use descriptive datacenter names:**
   ```bash
   -var="datacenter=dc-us-east-1"  # ✅ CLEAR
   ```

2. **Use environment-specific domains:**
   ```bash
   -var="domain=production.example.com"  # ✅ CLEAR
   ```

3. **Document variable values in README:**
   ```markdown
   ## Deployment Variables
   - `datacenter`: dc1 (local), dc-staging, dc-prod
   - `domain`: localhost, staging.example.com, app.example.com
   ```

### DON'T ❌

1. **Don't use "dc1" in production:**
   ```bash
   -var="datacenter=production"  # ✅ BETTER
   -var="datacenter=dc1"  # ❌ NOT DESCRIPTIVE
   ```

2. **Don't mix environments in same datacenter:**
   ```bash
   # ❌ BAD: staging and production in dc1
   -var="datacenter=dc1"  # for both

   # ✅ GOOD: separate datacenters
   -var="datacenter=dc-staging"  # for staging
   -var="datacenter=dc-production"  # for production
   ```

## 🎯 Success Criteria

- [ ] All 6 Nomad job templates parameterized with `datacenter` variable
- [ ] Test template parameterized with `domain` variable
- [ ] Unit tests for template validation with custom variables
- [ ] Backward compatibility verified (existing jobs still work)
- [ ] Documentation updated with variable usage examples
- [ ] Migration guide created

## 📚 References

- [Nomad Job Specification](https://developer.hashicorp.com/nomad/docs/job-specification)
- [Nomad Variables](https://developer.hashicorp.com/nomad/docs/job-specification/variable)
- [Consul Datacenters](https://developer.hashicorp.com/consul/docs/install/agent#datacenter-definition)

## 📝 Implementation Checklist

- [x] Analyze current hardcoded values in templates
- [x] Identify all templates with "dc1" and "localhost"
- [ ] Create CONTEXT.md (this file)
- [ ] Update deploy_web_service/web_service.nomad.hcl
- [ ] Update deploy_gpu_service/gpu_service.nomad.hcl
- [ ] Update deploy_traefik/traefik.nomad.hcl
- [ ] Update deploy_monitoring/monitoring.nomad.hcl
- [ ] Update deploy_backup_service/backup_service.nomad.hcl
- [ ] Update e2e_multi_node_scenarios/test-web-service.nomad.hcl
- [ ] Create unit tests for parameterized templates
- [ ] Test backward compatibility
- [ ] Update documentation
- [ ] Update tech-debt.md to mark TD-006 as complete

## 💡 Examples

### Example 1: Local Development

```bash
# Use defaults (dc1, localhost)
nomad job run web_service.nomad.hcl \
  -var="app_name=hello" \
  -var="image=nginx" \
  -var="host_rule=hello.localhost"

# Result:
# - Job runs in datacenter: dc1
# - Traefik routes hello.localhost → container
```

### Example 2: Staging Deployment

```bash
# Custom datacenter and domain
nomad job run web_service.nomad.hcl \
  -var="app_name=myapp-staging" \
  -var="image=mycompany/myapp:staging" \
  -var="datacenter=dc-staging" \
  -var="domain=staging.example.com" \
  -var="host_rule=myapp-staging.staging.example.com"

# Result:
# - Job runs in datacenter: dc-staging
# - Traefik routes myapp-staging.staging.example.com → container
```

### Example 3: Production Deployment

```bash
# Production datacenter and domain
nomad job run web_service.nomad.hcl \
  -var="app_name=myapp" \
  -var="image=mycompany/myapp:1.0.0" \
  -var="datacenter=dc-production" \
  -var="domain=app.example.com" \
  -var="host_rule=myapp.app.example.com"

# Result:
# - Job runs in datacenter: dc-production
# - Traefik routes myapp.app.example.com → container
```

### Example 4: Multi-Region Deployment

```bash
# Deploy to multiple regions
for region in us-east-1 us-west-2 eu-west-1; do
  nomad job run web_service.nomad.hcl \
    -var="app_name=myapp-${region}" \
    -var="datacenter=dc-${region}" \
    -var="domain=${region}.example.com" \
    -var="host_rule=myapp.${region}.example.com"
done

# Result:
# - 3 jobs deployed to 3 datacenters
# - Each accessible via region-specific domain
```
