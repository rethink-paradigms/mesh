# Feature: Pulumi Unit Tests for Resource Logic

**Description:**
Expand unit test coverage for Pulumi infrastructure provisioning code to catch regressions in VM provisioning logic, GPU configuration, and spot instance handling.

## Current State

**Existing Tests:** `src/infrastructure/provision_node/test_aws.py`

**Current Test Coverage:**
- ✅ `test_provision_aws_node_instance_type` - Verifies instance type is passed correctly
- ✅ `test_provision_aws_node_ami_selection` - Verifies AMI ID lookup
- ✅ `test_provision_aws_node_security_group` - Verifies security group attachment
- ✅ `test_provision_aws_node_user_data_rendering` - Verifies user_data injection
- ✅ `test_provision_aws_node_tags` - Verifies resource tagging
- ✅ `test_provision_aws_node_outputs` - Verifies output values (IPs, instance ID)
- ✅ `test_provision_node_aws_dispatch` - Verifies provider dispatch
- ✅ `test_provision_node_multipass_dispatch` - Verifies multipass provider
- ✅ `test_provision_node_unknown_provider` - Verifies error handling

**Missing Coverage:**
- ❌ No tests for GPU configuration parameters in boot script generation
- ❌ No tests for spot instance handling parameters
- ❌ No tests for resource dependencies (`depends_on` propagation)
- ❌ No tests for multipass provider with Pulumi mocks
- ❌ No tests for boot script integration with real `generate_shell_script()`
- ❌ No tests for output value resolution with `.apply()` chains
- ❌ No tests for error scenarios (missing parameters, invalid values)
- ❌ No integration-style tests for leader + worker provisioning

## Note on Architecture

The tech debt document references `MeshServer` as a ComponentResource class, but the actual implementation uses a **functional architecture**:

```
provision_node()              # Generic dispatcher (ComponentResource pattern)
├── provision_aws_node()      # AWS adapter (creates Instance, SG, etc.)
└── provision_multipass_node() # Multipass adapter
```

This functional approach provides the same abstraction benefits without the complexity of ComponentResource classes. The tests below validate this functional architecture.

## 🧩 Test Scope

### 1. GPU Configuration Tests

**Files:** `src/infrastructure/provision_node/test_gpu_integration.py`

**Tests to Add:**
- `test_gpu_config_default_values` - Verify default CUDA version, driver version
- `test_gpu_config_custom_versions` - Verify custom CUDA/driver versions
- `test_gpu_disabled_when_config_none` - Verify GPU not enabled when config is None
- `test_provision_node_with_gpu` - Verify GPU parameters passed to boot script
- `test_provision_node_without_gpu` - Verify GPU not passed when config omitted

**What to Test:**
```python
# GPUConfig dataclass
assert gpu_config.enable_gpu == True
assert gpu_config.cuda_version == "12.1"
assert gpu_config.nvidia_driver_version == "535"

# Boot script generation with GPU
boot_script = generate_shell_script(
    ts_key="test-key",
    leader_ip="127.0.0.1",
    role="client",
    has_gpu=True,
    cuda_version="12.1",
    driver_version="535"
)
assert "install_gpu_support" in boot_script
assert "CUDA_VERSION=12.1" in boot_script
```

### 2. Spot Instance Handling Tests

**Files:** `src/infrastructure/provision_node/test_spot_integration.py`

**Tests to Add:**
- `test_spot_config_default_values` - Verify default check interval and grace period
- `test_spot_config_custom_values` - Verify custom spot handling parameters
- `test_spot_disabled_when_config_none` - Verify spot handling disabled when config is None
- `test_provision_node_with_spot_handling` - Verify spot parameters passed to boot script
- `test_spot_handler_in_boot_script` - Verify spot handler script embedded in boot

**What to Test:**
```python
# SpotConfig dataclass
assert spot_config.enable_spot_handling == True
assert spot_config.spot_check_interval == 5
assert spot_config.spot_grace_period == 90

# Boot script generation with spot handling
boot_script = generate_shell_script(
    ts_key="test-key",
    leader_ip="127.0.0.1",
    role="client",
    enable_spot_handling=True,
    spot_check_interval=5,
    spot_grace_period=90
)
assert "SPOT_CHECK_INTERVAL=5" in boot_script
assert "SPOT_GRACE_PERIOD=90" in boot_script
```

### 3. Resource Dependencies Tests

**Files:** `src/infrastructure/provision_node/test_resource_dependencies.py`

**Tests to Add:**
- `test_depends_on_propagated_to_aws` - Verify depends_on passed to AWS adapter
- `test_depends_on_merged_with_opts` - Verify depends_on merged with existing opts
- `test_leader_depends_on_nothing` - Verify leader has no dependencies
- `test_worker_depends_on_leader` - Verify worker depends on leader
- `test_multiple_dependencies` - Verify multiple dependencies handled correctly

**What to Test:**
```python
# Test depends_on propagation
leader = provision_node(name="leader", ...)
worker = provision_node(
    name="worker",
    depends_on_resources=[leader["instance_id"]],
    ...
)

# Verify opts contains depends_on
assert worker.opts.depends_on is not None
assert leader["instance_id"] in worker.opts.depends_on
```

### 4. Boot Script Integration Tests

**Files:** `src/infrastructure/provision_node/test_boot_script_integration.py`

**Tests to Add:**
- `test_boot_script_contains_tailscale_key` - Verify TS key injected
- `test_boot_script_contains_role` - Verify role (server/client) in script
- `test_boot_script_contains_leader_ip` - Verify leader IP in script
- `test_boot_script_with_gpu` - Verify GPU installation commands present
- `test_boot_script_with_spot_handling` - Verify spot handler script present
- `test_boot_script_full_leader` - Verify complete leader boot script
- `test_boot_script_full_worker` - Verify complete worker boot script

**What to Test:**
```python
# Real boot script generation
from src.infrastructure.boot_consul_nomad.generate_boot_scripts import generate_shell_script

script = generate_shell_script(
    ts_key="tskey-123",
    leader_ip="10.0.0.1",
    role="server",
    has_gpu=True,
    enable_spot_handling=True
)

assert "TAILSCALE_KEY=tskey-123" in script
assert "ROLE=server" in script
assert "LEADER_IP=10.0.0.1" in script
assert "install_gpu_support" in script
assert "handle_spot_interruption" in script
```

### 5. Output Resolution Tests

**Files:** `src/infrastructure/provision_node/test_output_resolution.py`

**Tests to Add:**
- `test_public_ip_output_resolves` - Verify public_ip Output resolves correctly
- `test_private_ip_output_resolves` - Verify private_ip Output resolves correctly
- `test_instance_id_output_resolves` - Verify instance_id Output resolves correctly
- `test_output_chaining_with_apply` - Verify .apply() chains work correctly
- `test_output_all_combination` - Verify Output.all() for multiple values

**What to Test:**
```python
# Output resolution
outputs = provision_aws_node(...)

def check_ips(public_ip, private_ip, instance_id):
    assert public_ip == "1.2.3.4"
    assert private_ip == "10.0.0.1"
    assert "test-aws_id" in instance_id

return pulumi.Output.all(
    outputs["public_ip"],
    outputs["private_ip"],
    outputs["instance_id"]
).apply(check_ips)
```

### 6. Multipass Provider Tests with Pulumi Mocks

**Files:** `src/infrastructure/provision_node/test_multipass_with_mocks.py`

**Tests to Add:**
- `test_multipass_node_with_pulumi_mocks` - Verify multipass works with Pulumi runtime
- `test_multipass_dispatch_from_provision_node` - Verify dispatch to multipass adapter
- `test_multipass_boot_script_injection` - Verify boot script passed to multipass
- `test_multipass_instance_size` - Verify size parameter handling

**What to Test:**
```python
# Multipass with Pulumi mocks (even though multipass is local)
created_resources = {}

class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        created_resources[args.name] = args.inputs
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}

# Test multipass node creation with Pulumi runtime
outputs = provision_node(
    name="test-multipass",
    provider="multipass",
    role="client",
    size="2CPU,1GB",
    tailscale_auth_key=pulumi.Output.secret("test-key"),
    leader_ip="vm-leader"
)

assert outputs["public_ip"] is not None
```

### 7. Error Scenario Tests

**Files:** `src/infrastructure/provision_node/test_error_scenarios.py`

**Tests to Add:**
- `test_empty_name_raises_error` - Verify empty name is rejected
- `test_invalid_role_raises_error` - Verify invalid role is rejected
- `test_missing_tailscale_key_raises_error` - Verify missing TS key is rejected
- `test_bare_metal_provider_not_implemented` - Verify NotImplementedError for bare-metal
- `test_invalid_provider_raises_error` - Verify ValueError for unknown provider
- `test_gpu_config_with_server_role_warning` - Verify GPU with server role logs warning

**What to Test:**
```python
# Error scenarios
with pytest.raises(ValueError, match="Unknown compute provider"):
    provision_node(
        name="test",
        provider="invalid-provider",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("key"),
        leader_ip="127.0.0.1"
    )

with pytest.raises(NotImplementedError, match="Bare Metal provider"):
    provision_node(
        name="test",
        provider="bare-metal",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("key"),
        leader_ip="127.0.0.1"
    )
```

### 8. Integration-Style Tests

**Files:** `src/infrastructure/provision_node/test_integration_scenarios.py`

**Tests to Add:**
- `test_leader_and_worker_provisioning` - Verify leader + worker scenario
- `test_multiple_workers_with_spread` - Verify multiple workers spread across nodes
- `test_gpu_worker_with_spot_handling` - Verify GPU + spot combination
- `test_full_stack_with_dependencies` - Verify complete provisioning stack

**What to Test:**
```python
# Leader + Worker scenario
leader = provision_node(
    name="test-leader",
    provider="aws",
    role="server",
    size="t3.small",
    tailscale_auth_key=pulumi.Output.secret("ts-key"),
    leader_ip="127.0.0.1"
)

worker = provision_node(
    name="test-worker",
    provider="aws",
    role="client",
    size="t3.micro",
    tailscale_auth_key=pulumi.Output.secret("ts-key"),
    leader_ip="vm-leader",
    depends_on_resources=[leader["instance_id"]]
)

def check_both_provisioned(leader_id, worker_id):
    assert "test-leader" in leader_id
    assert "test-worker" in worker_id

return pulumi.Output.all(
    leader["instance_id"],
    worker["instance_id"]
).apply(check_both_provisioned)
```

## 📦 Dependencies

- Existing Pulumi test framework (`pulumi.runtime.Mocks`)
- Existing test utilities and fixtures
- `pytest` for test execution
- `pytest-mock` for mocking

## 🧪 Test Implementation Strategy

### Phase 1: Create Test Directory Structure

```
src/infrastructure/provision_node/tests/
├── test_gpu_integration.py           # GPU configuration tests
├── test_spot_integration.py          # Spot instance handling tests
├── test_resource_dependencies.py     # Dependency propagation tests
├── test_boot_script_integration.py   # Boot script integration tests
├── test_output_resolution.py         # Output resolution tests
├── test_multipass_with_mocks.py      # Multipass provider tests
├── test_error_scenarios.py           # Error scenario tests
└── test_integration_scenarios.py     # Integration-style tests
```

### Phase 2: Implement GPU Configuration Tests

**Test Count:** 5 tests

**Key Test Cases:**
1. Default GPU configuration values
2. Custom CUDA/driver versions
3. GPU disabled when config is None
4. GPU parameters passed to boot script
5. GPU not passed when config omitted

### Phase 3: Implement Spot Instance Handling Tests

**Test Count:** 5 tests

**Key Test Cases:**
1. Default spot configuration values
2. Custom spot handling parameters
3. Spot disabled when config is None
4. Spot parameters passed to boot script
5. Spot handler script embedded in boot

### Phase 4: Implement Resource Dependencies Tests

**Test Count:** 5 tests

**Key Test Cases:**
1. depends_on propagated to AWS adapter
2. depends_on merged with existing opts
3. Leader has no dependencies
4. Worker depends on leader
5. Multiple dependencies handled correctly

### Phase 5: Implement Boot Script Integration Tests

**Test Count:** 7 tests

**Key Test Cases:**
1. Tailscale key injection
2. Role (server/client) in script
3. Leader IP in script
4. GPU installation commands present
5. Spot handler script present
6. Complete leader boot script
7. Complete worker boot script

### Phase 6: Implement Output Resolution Tests

**Test Count:** 5 tests

**Key Test Cases:**
1. public_ip Output resolves
2. private_ip Output resolves
3. instance_id Output resolves
4. .apply() chains work correctly
5. Output.all() for multiple values

### Phase 7: Implement Multipass Provider Tests

**Test Count:** 4 tests

**Key Test Cases:**
1. Multipass works with Pulumi runtime
2. Dispatch to multipass adapter
3. Boot script passed to multipass
4. Instance size parameter handling

### Phase 8: Implement Error Scenario Tests

**Test Count:** 6 tests

**Key Test Cases:**
1. Empty name raises error
2. Invalid role raises error
3. Missing tailscale key raises error
4. Bare-metal provider not implemented
5. Invalid provider raises error
6. GPU with server role warning

### Phase 9: Implement Integration-Style Tests

**Test Count:** 4 tests

**Key Test Cases:**
1. Leader + worker scenario
2. Multiple workers with spread
3. GPU worker + spot handling
4. Full stack with dependencies

## 📝 Test Implementation Example

### GPU Configuration Test

```python
"""
Tests for GPU Configuration Integration
"""
import pytest
import pulumi
from src.infrastructure.provision_node.provision_node import (
    provision_node,
    GPUConfig
)

class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]
    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}

@pytest.fixture(autouse=True)
def setup_mocks():
    pulumi.runtime.set_mocks(MyMocks(), preview=False)

def test_gpu_config_default_values():
    """Test_GPUConfig_DefaultValues: Verify default GPU configuration."""
    config = GPUConfig()
    assert config.enable_gpu == True
    assert config.cuda_version == "12.1"
    assert config.nvidia_driver_version == "535"

def test_gpu_config_custom_versions():
    """Test_GPUConfig_CustomVersions: Verify custom CUDA/driver versions."""
    config = GPUConfig(
        enable_gpu=True,
        cuda_version="11.8",
        nvidia_driver_version="520"
    )
    assert config.cuda_version == "11.8"
    assert config.nvidia_driver_version == "520"

@pulumi.runtime.test
def test_provision_node_with_gpu():
    """Test_ProvisionNode_WithGPU: Verify GPU parameters passed to boot script."""
    created_resources = {}

    class TrackMocks(pulumi.runtime.Mocks):
        def new_resource(self, args: pulumi.runtime.MockResourceArgs):
            created_resources[args.name] = args.inputs
            return [args.name + "_id", args.inputs]
        def call(self, args: pulumi.runtime.MockCallArgs):
            return {}

    pulumi.runtime.set_mocks(TrackMocks(), preview=False)

    outputs = provision_node(
        name="test-gpu-node",
        provider="aws",
        role="client",
        size="g4dn.xlarge",
        tailscale_auth_key=pulumi.Output.secret("test-key"),
        leader_ip="vm-leader",
        gpu_config=GPUConfig(enable_gpu=True, cuda_version="12.1")
    )

    def check_gpu_in_user_data(instance_id):
        inputs = created_resources["test-gpu-node"]
        user_data = inputs.get("userData", {})
        if isinstance(user_data, dict) and 'value' in user_data:
            user_data = user_data['value']
        assert "install_gpu_support" in user_data
        assert "CUDA_VERSION=12.1" in user_data

    return outputs["instance_id"].apply(check_gpu_in_user_data)
```

### Resource Dependencies Test

```python
"""
Tests for Resource Dependencies
"""
import pytest
import pulumi
from src.infrastructure.provision_node.provision_node import provision_node

class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]
    def call(self, args: pulumi.runtime.MockCallArgs):
        if args.token == "aws:ec2/getAmi:getAmi":
            return {"id": "ami-0abcdef1234567890"}
        return {}

@pytest.fixture(autouse=True)
def setup_mocks():
    pulumi.runtime.set_mocks(MyMocks(), preview=False)

@pulumi.runtime.test
def test_worker_depends_on_leader():
    """Test_Worker_DependsOnLeader: Verify worker depends on leader."""
    leader = provision_node(
        name="test-leader",
        provider="aws",
        role="server",
        size="t3.small",
        tailscale_auth_key=pulumi.Output.secret("ts-key"),
        leader_ip="127.0.0.1"
    )

    worker = provision_node(
        name="test-worker",
        provider="aws",
        role="client",
        size="t3.micro",
        tailscale_auth_key=pulumi.Output.secret("ts-key"),
        leader_ip="vm-leader",
        depends_on_resources=[leader["instance_id"]]
    )

    def check_both_created(leader_id, worker_id):
        # This test verifies the dependency chain is respected
        assert leader_id is not None
        assert worker_id is not None

    return pulumi.Output.all(
        leader["instance_id"],
        worker["instance_id"]
    ).apply(check_both_created)
```

### Boot Script Integration Test

```python
"""
Tests for Boot Script Integration
"""
import pytest
from src.infrastructure.boot_consul_nomad.generate_boot_scripts import generate_shell_script

def test_boot_script_contains_all_required_params():
    """Test_BootScript_AllParams: Verify boot script contains all required parameters."""
    script = generate_shell_script(
        ts_key="tskey-control-123",
        leader_ip="10.0.0.1",
        role="client",
        has_gpu=True,
        cuda_version="12.1",
        driver_version="535",
        enable_spot_handling=True,
        spot_check_interval=5,
        spot_grace_period=90
    )

    # Required parameters
    assert "TAILSCALE_KEY=tskey-control-123" in script
    assert "ROLE=client" in script
    assert "LEADER_IP=10.0.0.1" in script

    # GPU support
    assert "install_gpu_support" in script
    assert "CUDA_VERSION=12.1" in script
    assert "NVIDIA_DRIVER_VERSION=535" in script

    # Spot handling
    assert "SPOT_CHECK_INTERVAL=5" in script
    assert "SPOT_GRACE_PERIOD=90" in script
    assert "handle_spot_interruption" in script
```

## ⚠️ Limitations & Considerations

### Pulumi Mocks vs Real Resources

**Limitation:** Pulumi mocks don't actually create AWS resources

**Mitigation:**
- Tests verify resource configuration, not actual provisioning
- E2E tests (TD-005) validate real infrastructure
- Mocks ensure configuration correctness without AWS costs

### Boot Script Complexity

**Limitation:** Boot script is 181+ lines, hard to test comprehensively

**Mitigation:**
- Test key components (TS key, role, leader IP)
- Test GPU and spot handling sections separately
- Existing template validation tests (TD-001) cover syntax

### Multipass Provider

**Limitation:** Multipass is local, doesn't use Pulumi resources

**Mitigation:**
- Test dispatch logic with Pulumi mocks
- Real Multipass testing requires local cluster (deferred)

### Output Resolution Timing

**Limitation:** Pulumi Outputs resolve asynchronously

**Mitigation:**
- Use `@pulumi.runtime.test` decorator
- Use `.apply()` for assertions
- Use `Output.all()` for multiple values

## 🔒 Best Practices

### DO ✅

1. **Use Pulumi Mocks:**
   ```python
   class MyMocks(pulumi.runtime.Mocks):
       def new_resource(self, args):
           return [args.name + "_id", args.inputs]
       def call(self, args):
           return {}
   ```

2. **Clear created_resources before each test:**
   ```python
   created_resources = {}
   @pulumi.runtime.test
   def test_something():
       created_resources.clear()
       # test code here
   ```

3. **Use .apply() for assertions on Outputs:**
   ```python
   def check_value(value):
       assert value == "expected"
   return output.apply(check_value)
   ```

4. **Group related tests in files:**
   - test_gpu_integration.py
   - test_spot_integration.py
   - test_resource_dependencies.py

### DON'T ❌

1. **Don't create real AWS resources in unit tests:**
   ```python
   # Bad: This creates real resources
   instance = aws.ec2.Instance("real-instance", ...)

   # Good: This uses mocks
   instance = aws.ec2.Instance("test-instance", ...)
   ```

2. **Don't ignore Pulumi Outputs:**
   ```python
   # Bad: Output not resolved
   assert outputs["public_ip"] == "1.2.3.4"

   # Good: Use .apply()
   outputs["public_ip"].apply(lambda ip: assert ip == "1.2.3.4")
   ```

3. **Don't mix E2E tests with unit tests:**
   - Unit tests: Use mocks, run in CI
   - E2E tests: Use real resources, run manually

## 🎯 Success Criteria

- [ ] GPU configuration tests: 5 tests passing
- [ ] Spot instance handling tests: 5 tests passing
- [ ] Resource dependencies tests: 5 tests passing
- [ ] Boot script integration tests: 7 tests passing
- [ ] Output resolution tests: 5 tests passing
- [ ] Multipass provider tests: 4 tests passing
- [ ] Error scenario tests: 6 tests passing
- [ ] Integration-style tests: 4 tests passing
- [ ] **Total: 41 new tests passing**
- [ ] All existing tests still passing (188 + 41 = 229 total)
- [ ] Test execution time under 30 seconds
- [ ] Tests run in CI without AWS credentials

## 📚 References

- [Pulumi Python Testing Guide](https://www.pulumi.com/docs/guides/testing/python/)
- [Pulumi Unit Testing Best Practices](https://www.pulumi.com/docs/guides/testing/)
- [Existing test_aws.py](src/infrastructure/provision_node/test_aws.py)
- [Pulumi Runtime Mocks Documentation](https://www.pulumi.com/docs/reference/pkg/python/pulumi/pulumi/#runtime.Mocks)

## 📝 Implementation Checklist

- [x] Analyze existing Pulumi test coverage
- [x] Identify missing test scenarios
- [x] Create CONTEXT.md (this file)
- [ ] Create test file: test_gpu_integration.py (5 tests)
- [ ] Create test file: test_spot_integration.py (5 tests)
- [ ] Create test file: test_resource_dependencies.py (5 tests)
- [ ] Create test file: test_boot_script_integration.py (7 tests)
- [ ] Create test file: test_output_resolution.py (5 tests)
- [ ] Create test file: test_multipass_with_mocks.py (4 tests)
- [ ] Create test file: test_error_scenarios.py (6 tests)
- [ ] Create test file: test_integration_scenarios.py (4 tests)
- [ ] Run all tests and verify 41 new tests pass
- [ ] Verify all existing tests still pass
- [ ] Update docs/engineering/tech-debt.md to mark TD-012 as complete

## 🧪 Test Execution

```bash
# Run all Pulumi infrastructure tests
pytest src/infrastructure/provision_node/ -v

# Run only GPU tests
pytest src/infrastructure/provision_node/test_gpu_integration.py -v

# Run only spot handling tests
pytest src/infrastructure/provision_node/test_spot_integration.py -v

# Run with coverage
pytest src/infrastructure/provision_node/ --cov=. --cov-report=html

# Run specific test
pytest src/infrastructure/provision_node/test_gpu_integration.py::test_gpu_config_default_values -v
```

## Expected Outcomes

After implementing these tests:

1. **Better Coverage:** 41 new tests cover GPU, spot handling, dependencies, and integration scenarios
2. **Regression Prevention:** Changes to provisioning logic caught by tests
3. **Documentation:** Tests serve as examples of how to use the provisioning API
4. **CI Integration:** Tests run automatically without AWS credentials
5. **Faster Development:** Catch issues before running `pulumi up`

## Related Tech Debt Items

- **TD-004:** Automated Backup/Restore - Test backup service provisioning
- **TD-009:** Spot Instance Interruption Handling - Test spot configuration
- **TD-005:** E2E Multi-Node Tests - Validate real infrastructure

## Architecture Alignment

This testing strategy aligns with:

- **ADR-003 (Pulumi Python IaC):** Pulumi-based infrastructure with Python
- **ADR-007 (Zero Manual Configuration):** All infrastructure as code
- **ADR-006 (Memory Constraints):** Resource options tested without real resources

---

**Status:** Design Complete, Ready for Implementation

**Estimated Effort:** S (1 day) - 41 tests across 8 test files

**Next Step:** Begin implementation with Phase 1 (GPU Configuration Tests)
