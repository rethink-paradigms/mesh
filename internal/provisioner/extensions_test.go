package provisioner

import (
	"context"
	"io"
	"testing"
)

// fullMockProvisioner implements ProvisionerAdapter and all 3 extension interfaces.
type fullMockProvisioner struct{}

func (m *fullMockProvisioner) CreateMachine(ctx context.Context, spec MachineSpec, userData string) (MachineID, error) {
	return "", nil
}
func (m *fullMockProvisioner) DestroyMachine(ctx context.Context, id MachineID) error {
	return nil
}
func (m *fullMockProvisioner) GetMachineStatus(ctx context.Context, id MachineID) (MachineStatus, error) {
	return MachineStatus{}, nil
}
func (m *fullMockProvisioner) ListMachines(ctx context.Context) ([]MachineInfo, error) {
	return nil, nil
}
func (m *fullMockProvisioner) Name() string {
	return "full-mock"
}
func (m *fullMockProvisioner) IsHealthy(ctx context.Context) bool {
	return true
}
func (m *fullMockProvisioner) SnapshotMachine(ctx context.Context, id MachineID) (io.ReadCloser, error) {
	return nil, nil
}
func (m *fullMockProvisioner) ConfigureNetwork(ctx context.Context, id MachineID, spec NetworkSpec) error {
	return nil
}
func (m *fullMockProvisioner) GetMachineLogs(ctx context.Context, id MachineID) (string, error) {
	return "", nil
}

// minimalMockProvisioner implements only ProvisionerAdapter (no extensions).
type minimalMockProvisioner struct{}

func (m *minimalMockProvisioner) CreateMachine(ctx context.Context, spec MachineSpec, userData string) (MachineID, error) {
	return "", nil
}
func (m *minimalMockProvisioner) DestroyMachine(ctx context.Context, id MachineID) error {
	return nil
}
func (m *minimalMockProvisioner) GetMachineStatus(ctx context.Context, id MachineID) (MachineStatus, error) {
	return MachineStatus{}, nil
}
func (m *minimalMockProvisioner) ListMachines(ctx context.Context) ([]MachineInfo, error) {
	return nil, nil
}
func (m *minimalMockProvisioner) Name() string {
	return "minimal-mock"
}
func (m *minimalMockProvisioner) IsHealthy(ctx context.Context) bool {
	return true
}

func TestProvisionerExtensions(t *testing.T) {
	var full ProvisionerAdapter = &fullMockProvisioner{}
	var minimal ProvisionerAdapter = &minimalMockProvisioner{}

	if _, ok := full.(Snapshotter); !ok {
		t.Error("fullMockProvisioner should implement Snapshotter")
	}
	if _, ok := full.(NetworkConfigurator); !ok {
		t.Error("fullMockProvisioner should implement NetworkConfigurator")
	}
	if _, ok := full.(LogFetcher); !ok {
		t.Error("fullMockProvisioner should implement LogFetcher")
	}

	if _, ok := minimal.(Snapshotter); ok {
		t.Error("minimalMockProvisioner should NOT implement Snapshotter")
	}
	if _, ok := minimal.(NetworkConfigurator); ok {
		t.Error("minimalMockProvisioner should NOT implement NetworkConfigurator")
	}
	if _, ok := minimal.(LogFetcher); ok {
		t.Error("minimalMockProvisioner should NOT implement LogFetcher")
	}
}

func TestHasCapabilitySnapshotter(t *testing.T) {
	var full ProvisionerAdapter = &fullMockProvisioner{}
	var minimal ProvisionerAdapter = &minimalMockProvisioner{}

	if !HasCapability[Snapshotter](full) {
		t.Error("HasCapability[Snapshotter](fullMock) should return true")
	}
	if HasCapability[Snapshotter](minimal) {
		t.Error("HasCapability[Snapshotter](minimalMock) should return false")
	}
}

func TestHasCapabilityLogFetcher(t *testing.T) {
	var full ProvisionerAdapter = &fullMockProvisioner{}

	if !HasCapability[LogFetcher](full) {
		t.Error("HasCapability[LogFetcher](fullMock) should return true")
	}
}

func TestNetworkConfiguratorInterface(t *testing.T) {
	var full ProvisionerAdapter = &fullMockProvisioner{}
	nc, ok := full.(NetworkConfigurator)
	if !ok {
		t.Fatal("fullMock should implement NetworkConfigurator")
	}

	ctx := context.Background()
	err := nc.ConfigureNetwork(ctx, "machine-1", NetworkSpec{CIDR: "10.0.0.0/24", Gateway: "10.0.0.1"})
	if err != nil {
		t.Errorf("ConfigureNetwork returned unexpected error: %v", err)
	}
}
