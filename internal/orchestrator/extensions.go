package orchestrator

import (
	"context"
	"io"
	"time"
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ContainerMetadata struct {
	Image    string
	Env      map[string]string
	Cmd      []string
	Workdir  string
	Platform string
}

type NodeInfo struct {
	ID         string
	Name       string
	Address    string
	State      string // "ready", "down", "initializing"
	Capacity   NodeCapacity
	Provider   string // empty for MVP
	Region     string // empty for MVP
	LastSeenAt time.Time
}

type NodeCapacity struct {
	CPUMHZ   int
	MemoryMB int
	DiskGB   int
}

type Allocation struct {
	ID     string
	JobID  string
	NodeID string
	State  string
	Ports  []AllocPort
}

type AllocPort struct {
	Label    string
	HostPort int
}

type Exporter interface {
	ExportFilesystem(ctx context.Context, id Handle) (io.ReadCloser, error)
}

type Importer interface {
	ImportFilesystem(ctx context.Context, id Handle, tarball io.Reader) error
}

type Inspector interface {
	Inspect(ctx context.Context, id Handle) (ContainerMetadata, error)
}

type Executor interface {
	Exec(ctx context.Context, id Handle, cmd []string) (ExecResult, error)
}

type NodeLister interface {
	ListNodes(ctx context.Context) ([]NodeInfo, error)
}

type AllocQuerier interface {
	GetAllocations(ctx context.Context, jobID string) ([]Allocation, error)
}

func HasCapability[T any](adapter OrchestratorAdapter) bool {
	_, ok := adapter.(T)
	return ok
}
