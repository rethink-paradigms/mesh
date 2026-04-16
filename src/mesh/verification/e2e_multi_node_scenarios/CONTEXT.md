# Feature: Multi-Node E2E Test Scenarios

**Description:**
End-to-end tests that validate production-like multi-node cluster scenarios including scheduling, fault tolerance, and cross-cloud mesh networking.

## Current State (Single-Node Tests)

**Existing Tests:** `src/verification/e2e_app_deployment/`
- `test_marketing_site_deployment` - Deploys app to single node
- Validates HTTP access via Traefik ingress
- Tests content verification
- 60-second timeout loop

**Limitations:**
- Only validates single-node scenarios
- No multi-node scheduling validation
- No fault tolerance testing
- No cross-cloud connectivity testing

## 🧩 New Test Scenarios

### Scenario 1: Multi-Node Scheduling (Leader + 2 Workers)

**Test Name:** `test_multi_node_app_scheduling`

**Purpose:** Verify Nomad schedules workloads across multiple worker nodes correctly.

**Setup:**
- 1 Leader node (runs Nomad/Consul server + Traefik)
- 2 Worker nodes (run Nomad/Consul client)
- Deploy a job with 3 identical task groups

**Expected Behavior:**
1. Deploy job with 3 replicas
2. Verify each replica scheduled to different worker (1 per worker)
3. Verify leader can access all replicas via Traefik
4. Verify Consul service registration for all replicas

**Validation:**
- `nomad job status` shows 3 allocations, spread across nodes
- `consul catalog services` shows 3 healthy instances
- HTTP requests to each replica succeed

**Failure Modes:**
- All replicas scheduled to single worker → Nomad spread not working
- Some replicas pending → Insufficient resources or worker unavailable
- Traefik can't reach replicas → Service discovery broken

---

### Scenario 2: Worker Failure & Automatic Rescheduling

**Test Name:** `test_worker_failure_rescheduling`

**Purpose:** Verify Nomad automatically reschedules workloads when a worker fails.

**Setup:**
- 1 Leader + 2 Workers
- Deploy job with 2 replicas (1 per worker)
- Simulate worker failure (stop Nomad client)

**Expected Behavior:**
1. Initial state: 2 replicas running on Worker-1 and Worker-2
2. Stop Nomad client on Worker-1
3. Nomad detects Worker-1 as unavailable
4. Replica from Worker-1 rescheduled to Worker-2
5. Verify Consul service health updates
6. Verify Traefik routes to remaining healthy instances

**Validation:**
- `nomad node status` shows Worker-1 as "down"
- `nomad alloc status` shows rescheduled allocation on Worker-2
- Consul health checks show only healthy instances
- HTTP requests still succeed (no 502/503 errors)

**Cleanup:**
- Restart Nomad client on Worker-1
- Verify node rejoins cluster
- Verify workload distribution rebalances (optional)

**Failure Modes:**
- Replica not rescheduled → Nomad fault tolerance broken
- Stale service registrations → Consul not updating health
- Traefik routing to failed worker → Service discovery lag

---

### Scenario 3: Cross-Cloud Mesh (AWS + Hetzner)

**Test Name:** `test_cross_cloud_mesh_connectivity`

**Purpose:** Verify Tailscale mesh networking enables cross-cloud communication.

**Setup:**
- 1 Leader on AWS (us-east-1)
- 1 Worker on Hetzner (nbg1)
- Both join same Tailscale tailnet
- Deploy app on Hetzner worker
- Ingress via Traefik on AWS leader

**Expected Behavior:**
1. Both nodes boot and join Tailscale mesh
2. Verify Tailscale IP connectivity (ping leader ↔ worker)
3. Nomad/Consul cluster forms across clouds
4. Deploy job with placement constraint {region = "hetzner"}
5. App scheduled on Hetzner worker
6. HTTP request from AWS leader reaches Hetzner app via Traefik

**Validation:**
- `tailscale status` shows both nodes in mesh
- `consul members` shows both nodes (1 AWS, 1 Hetzner)
- `nomad node status` shows both nodes eligible
- App deployment succeeds on Hetzner worker
- HTTP request from AWS → Hetzner app succeeds

**Failure Modes:**
- Nodes can't communicate → Tailscale misconfigured
- Cluster split → Consul/Nomad can't form quorum
- App unreachable → Network routing issues

---

### Scenario 4: Multi-Cluster Service Discovery

**Test Name:** `test_multi_node_service_discovery`

**Purpose:** Verify Consul service discovery works across multiple nodes.

**Setup:**
- 1 Leader + 2 Workers
- Deploy 2 different services (api, frontend)
- Frontend configured to discover API via Consul DNS

**Expected Behavior:**
1. API service deployed (2 replicas across workers)
2. Frontend deployed (1 replica)
3. Frontend queries Consul DNS: `api.service.consul`
4. DNS returns IPs of both API replicas
5. Frontend can connect to API instances

**Validation:**
- `dig api.service.consul` returns both worker IPs
- Service health checks pass for all instances
- Load balancing distributes requests across replicas
- Failure of one API instance doesn't break frontend

**Failure Modes:**
- DNS queries fail → Consul DNS not configured
- Only one IP returned → Service discovery incomplete
- Connections fail → Network or health check issues

---

### Scenario 5: Traefik Ingress Routing

**Test Name:** `test_traefik_routing_after_deployment`

**Purpose:** Verify HTTP requests route correctly through Traefik ingress to a deployed workload.

**Setup:**
- 1 Leader node (runs Nomad/Consul server + Traefik)
- Deploy test-web-service with Traefik v3 Consul Catalog tags

**Expected Behavior:**
1. Deploy test-web-service (1 replica)
2. Wait for allocation to be running
3. Send HTTP GET to leader IP with Host header matching service
4. Assert HTTP 200 response from Traefik
5. Verify Consul service registration

**Validation:**
- Traefik v3 Consul Catalog tags correctly configured
- HTTP request with correct Host header returns 200
- Consul catalog shows registered service instance

**Failure Modes:**
- HTTP 502/503 → Traefik can't reach backend service
- HTTP 404 → Traefik router rule not matching Host header
- No Consul registration → Service discovery broken

---

## 🔧 Test Implementation Details

### Prerequisites

**Environment Variables:**
```bash
# For local testing (Multipass)
export E2E_TARGET_ENV="local"
export E2E_LEADER_IP=$(multipass info local-leader --format json | jq -r '.info["local-leader"].ipv4[0]')

# For cloud testing (AWS)
export E2E_TARGET_ENV="aws"
export E2E_LEADER_IP=$(pulumi stack output leader_public_ip)
export E2E_WORKER_IPS=$(pulumi stack output workers_public_ips)

# For cross-cloud testing
export E2E_CROSS_CLOUD="true"
export E2E_AWS_LEADER=$(pulumi stack output aws_leader_public_ip)
export E2E_HETZNER_WORKER=$(pulumi stack output hetzner_worker_public_ip)
```

**Cluster State:**
- E2E tests assume cluster is already provisioned
- Use `provision_local_cluster/cli.py up` for local testing
- Use `provision_cloud_cluster/pulumi up` for cloud testing

### Test Utilities

**File:** `src/verification/e2e_multi_node_scenarios/test_utils.py`

**Helper Functions:**
```python
def get_cluster_nodes() -> List[Dict]:
    """Get list of all cluster nodes (leader + workers)"""

def deploy_job(job_file: str, vars: Dict) -> str:
    """Deploy Nomad job and return job ID"""

def wait_for_allocation(job_id: str, timeout: int = 120) -> bool:
    """Wait for job allocations to be running"""

def get_allocation_nodes(job_id: str) -> Dict[str, str]:
    """Return mapping of allocation ID to node name"""

def stop_nomad_client(node_ip: str):
    """Stop Nomad client on specific node (simulate failure)"""

def start_nomad_client(node_ip: str):
    """Start Nomad client on specific node (recovery)"""

def verify_service_discovery(service_name: str) -> List[str]:
    """Query Consul DNS and return list of service IPs"""

def check_tailscale_mesh() -> bool:
    """Verify Tailscale mesh connectivity"""

def check_traefik_routing(leader_ip: str, host_header: str, port: int = 80, timeout: int = 10) -> requests.Response:
    """Send HTTP GET through Traefik ingress and return response"""
```

### Mocking Strategy

**Local Development:**
- Mock cluster provisioning (don't actually create VMs)
- Mock Nomad API responses
- Mock Consul catalog queries

**CI/CD Pipeline:**
- Use actual Multipass cluster for fast feedback
- Skip cloud E2E tests (too slow) - run in nightly builds only

### Test Isolation

**Job Namespacing:**
- Each test uses unique job name: `e2e-test-{test_name}-{timestamp}`
- Prevents conflicts between concurrent tests

**Cleanup:**
- Each test cleans up deployed jobs: `nomad job stop -purge`
- Tests marked as "destructive" run sequentially
- Non-destructive tests can run in parallel

## ⚙️ Configuration

### Test Markers

```python
@pytest.mark.e2e
def test_something():
    """Run only when --run-e2e flag is set"""

@pytest.mark.local_only
def test_local_cluster():
    """Skip in cloud environments"""

@pytest.mark.cloud_only
def test_cloud_infrastructure():
    """Skip in local environment"""

@pytest.mark.slow
def test_cross_cloud():
    """Skip in quick test runs"""
```

**Usage:**
```bash
# Run all E2E tests
pytest -m e2e --run-e2e

# Run only local E2E tests
pytest -m "e2e and local_only" --run-e2e

# Skip slow tests
pytest -m "e2e and not slow" --run-e2e
```

### Test Data

**Sample Nomad Jobs:**
- `jobs/test-web-service.nomad.hcl` - Simple web service for scheduling tests
- `jobs/test-api.nomad.hcl` - API service for service discovery tests
- `jobs/test-multi-region.nomad.hcl` - Job with placement constraints

## 📋 Test Coverage Goals

| Scenario | Status | Priority |
|----------|--------|----------|
| Multi-node scheduling | New | HIGH |
| Worker failure & rescheduling | New | HIGH |
| Cross-cloud mesh | New | MEDIUM |
| Multi-cluster service discovery | New | MEDIUM |
| Traefik ingress routing | New | HIGH |

## ⚠️ Limitations

### Cloud Tests
- **Cost:** Cloud E2E tests require actual AWS/Hetzner resources
- **Duration:** Cross-cloud tests can take 10-20 minutes (VM boot time)
- **Recommendation:** Run in nightly builds, not per-commit

### Local Tests
- **Multipass Only:** Local tests currently only support Multipass
- **macOS Only:** Multipass is macOS-only (Linux support via LXD future)

### Destructive Tests
- **Worker Failure Tests:** Stop Nomad client, may affect concurrent tests
- **Must Run Sequentially:** Mark tests as destructive to enforce serial execution

## 🔗 Related Features

- **provision_local_cluster:** Creates local Multipass cluster for E2E testing
- **provision_cloud_cluster:** Creates AWS/Hetzner cluster for production E2E tests
- **boot_consul_nomad:** Cluster formation and mesh networking
- **deploy_web_service:** Job templates used in E2E tests

## 📚 References

- [Nomad Testing Best Practices](https://www.nomadproject.io/guides/testing.html)
- [Consul Testing Strategies](https://www.consul.io/docs/install/testing)
- [Tailscale Mocking for Tests](https://tailscale.com/kb/1150/faux-addr)

## 🎯 Success Criteria

- [ ] Multi-node scheduling test validates spread across workers
- [ ] Worker failure test validates automatic rescheduling
- [ ] Cross-cloud test validates Tailscale mesh connectivity
- [ ] Service discovery test validates Consul DNS
- [ ] All tests pass against local Multipass cluster
- [ ] Tests documented with clear setup/teardown procedures
- [ ] CI/CD integration (skip E2E tests by default)

## 📝 Implementation Checklist

- [ ] Create `src/verification/e2e_multi_node_scenarios/CONTEXT.md` (this file)
- [ ] Create `test_utils.py` with helper functions
- [ ] Implement `test_multi_node_app_scheduling`
- [ ] Implement `test_worker_failure_rescheduling`
- [ ] Implement `test_cross_cloud_mesh_connectivity`
- [ ] Implement `test_multi_node_service_discovery`
- [ ] Implement `test_traefik_routing_after_deployment`
- [ ] Create test job templates (test-web-service.nomad.hcl, etc.)
- [ ] Update CI/CD configuration to handle E2E tests
- [ ] Document E2E test run procedures in README
