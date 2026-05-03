package provisioner

import (
	"context"
	"io"
)

// NetworkSpec defines the desired network configuration for a machine.
type NetworkSpec struct {
	CIDR    string
	Gateway string
}

// Snapshotter is an extension interface for adapters that support
// creating filesystem snapshots of machines.
type Snapshotter interface {
	SnapshotMachine(ctx context.Context, id MachineID) (io.ReadCloser, error)
}

// NetworkConfigurator is an extension interface for adapters that support
// configuring network settings on machines.
type NetworkConfigurator interface {
	ConfigureNetwork(ctx context.Context, id MachineID, spec NetworkSpec) error
}

// LogFetcher is an extension interface for adapters that support
// fetching logs from machines.
type LogFetcher interface {
	GetMachineLogs(ctx context.Context, id MachineID) (string, error)
}

// HasCapability reports whether the given adapter implements the
// extension interface T via type assertion.
func HasCapability[T any](adapter ProvisionerAdapter) bool {
	_, ok := adapter.(T)
	return ok
}
