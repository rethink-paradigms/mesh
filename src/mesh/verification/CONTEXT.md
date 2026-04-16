# Domain: System Testing

**Description:**
System testing that validates the complete flow of infrastructure provisioning and application deployment, from single-node scenarios to multi-node fault tolerance.

## 🧩 Public Interface

| Feature | Input | Output | Description |
|:---|:---|:---|:---|
| `E2EAppDeployment` | target_env, leader_ip, [app_name], [image], [port] | status | Black-box test of full deployment flow |
| `MultiNodeScenarios` | leader_ip, [worker_ips], [test_job] | test_results | Tests scheduling, fault tolerance, mesh connectivity |
| `E2ELiteMode` | leader_ip, [app_name], [image], [domain] | test_results | End-to-end testing for lite mode HTTPS ingress with Caddy |

**Parameters:**
- `target_env`: "local" (Multipass) or "aws" (Pulumi stack)
- `leader_ip`: Public IP of the cluster leader node
- `worker_ips`: Optional list of worker node IPs for multi-node tests
- `test_job`: Nomad job file to deploy for testing

## 📦 Dependencies

- **pytest** - Test framework
- **requests** - HTTP client for deployment verification
- **Nomad CLI** - Job deployment and status queries
- **Consul API** - Service discovery verification
- **Multipass CLI** - Local VM management (for local tests)
- **Tailscale** - Mesh connectivity verification (for cross-cloud tests)

## 🏗 Features

### `e2e_app_deployment/` - Single-Node E2E Tests
**Purpose:** Verifies the complete flow of deploying a web application to the running cluster and accessing it via the Traefik ingress controller.

**Key Files:**
- `test_e2e_deploy.py` - Main E2E test scenarios
- `test_verification_logic.py` - Verification helper functions
- `CONTEXT.md` - Feature documentation

**Test Scenarios:**
- `test_marketing_site_deployment` - Deploys app and verifies HTTP 200 with expected content
- `test_ingress_routing` - Verifies Host header routing to specific services
- `test_service_discovery` - Validates Consul service registration
- `test_health_checks` - Verifies Consul health check integration

**Test Flow:**
1. Deploy Nomad job with test application
2. Wait for allocation to become running
3. Query Traefik ingress via Leader public IP
4. Verify HTTP response status and content
5. Validate Consul service registration
6. Cleanup test deployment

**Requirements:**
- Running cluster (local Multipass or AWS)
- Nomad API accessible at `http://{leader_ip}:4646`
- Traefik accessible at `http://{leader_ip}:80`

### `e2e_lite_mode/` - Lite Mode E2E Tests
**Purpose:** End-to-end testing for lite mode HTTPS ingress with Caddy.

**Test Scenarios:**
- Single-node boot verification
- HTTPS certificate provisioning
- Multi-app routing
- Memory budget validation

**Status:** In Development

### `e2e_multi_node_scenarios/` - Multi-Node E2E Tests
**Purpose:** Validates production-like multi-node cluster scenarios including scheduling, fault tolerance, and cross-cloud mesh networking.

**Key Files:**
- `test_multi_node_scenarios.py` - 4 E2E test scenarios
- `test_utils.py` - Helper functions for cluster operations
- `test_utils_unit_tests.py` - 30 unit tests for utilities
- `test-web-service.nomad.hcl` - Test job template
- `CONTEXT.md` - Complete E2E test design documentation

**Test Scenarios:**

1. **test_multi_node_app_scheduling**
   - Verifies workload distribution across multiple worker nodes
   - Deploys 2 instances and validates they run on different nodes
   - Validates Nomad scheduler behavior

2. **test_worker_failure_rescheduling**
   - Simulates worker node failure (stops Nomad client)
   - Validates automatic rescheduling to remaining nodes
   - Verifies workload migration completes within timeout

3. **test_cross_cloud_mesh_connectivity**
   - Validates Tailscale mesh networking across providers
   - Tests node discovery via Tailscale MagicDNS
   - Verifies cross-cloud communication

4. **test_multi_node_service_discovery**
   - Validates Consul service discovery across nodes
   - Tests DNS queries for multi-node services
   - Verifies health check aggregation

**Test Utilities:**
- `ClusterConfig` - Configuration management for test environments
- `get_cluster_nodes()` - Discover cluster nodes
- `deploy_job()` - Deploy Nomad jobs
- `wait_for_allocation()` - Wait for job allocations
- `get_allocation_nodes()` - Map allocations to nodes
- `stop_nomad_client()` / `start_nomad_client()` - Simulate worker failure
- `verify_service_discovery()` - Query Consul DNS
- `check_tailscale_mesh()` - Verify mesh connectivity
- `cleanup_job()` - Test cleanup

**Environment Setup:**
```bash
# Local Multipass cluster
cd src/infrastructure/provision_local_cluster
python3 cli.py up  # Start cluster

# Run E2E tests
export E2E_TARGET_ENV=local
pytest src/verification/e2e_multi_node_scenarios/ -v

# AWS cluster
export E2E_LEADER_IP=1.2.3.4
pytest src/verification/e2e_multi_node_scenarios/ -v
```

### `test_app/` - Test Application
**Purpose:** Simple test application for E2E validation.

**Key Files:**
- `deploy.sh` - Deployment script for test application
- Test application container with static content

## 🧪 Test Coverage

**Unit Tests:**
- **30 tests** in `e2e_multi_node_scenarios/test_utils_unit_tests.py`

**E2E Tests:**
- **4 scenarios** in `e2e_multi_node_scenarios/test_multi_node_scenarios.py`
- **4 scenarios** in `e2e_app_deployment/test_e2e_deploy.py`

**Total:** ~60 tests (including unit tests for test utilities)

**Test Categories:**
- Scheduling validation
- Fault tolerance
- Cross-cloud mesh networking
- Service discovery
- Ingress routing
- Health checks

## 🔗 Dependencies on Other Domains

- **infrastructure/provision_node** - Tests run on nodes provisioned by this feature
- **infrastructure/provision_local_cluster** - Local cluster for testing
- **infrastructure/provision_cloud_cluster** - AWS cluster for testing
- **infrastructure/boot_consul_nomad** - Validates Nomad/Consul installation
- **infrastructure/configure_tailscale** - Validates mesh networking
- **workloads/deploy_web_service** - Uses web service templates for testing

## 📝 Design Principles

- **Black-Box Testing:** Tests external behavior, not internal implementation
- **Environment Agnostic:** Same tests work on local Multipass and AWS
- **Idempotent:** Tests can be run multiple times without side effects
- **Self-Cleaning:** Tests cleanup deployed jobs after completion
- **Graceful Skipping:** Tests skip when cluster unavailable

## 🚨 Test Execution Notes

**E2E Tests Require:**
- Actual cluster (local Multipass or AWS)
- Network connectivity to cluster nodes
- Nomad API accessible
- Tests take 2-5 minutes to complete

**Unit Tests:**
- Can run without cluster
- Complete in seconds
- Validate test utility functions

**Running Tests:**
```bash
# Skip E2E tests by default (require cluster)
pytest src/ -v

# Run only unit tests
pytest src/verification/e2e_multi_node_scenarios/test_utils_unit_tests.py -v

# Run E2E tests with cluster
export E2E_LEADER_IP=1.2.3.4
pytest src/verification/e2e_multi_node_scenarios/ -v

# Run E2E tests with local Multipass cluster
cd src/infrastructure/provision_local_cluster
python3 cli.py up
export E2E_TARGET_ENV=local
pytest src/verification/e2e_multi_node_scenarios/ -v
```
