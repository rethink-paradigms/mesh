package orchestrator

import (
	"context"
	"io"
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

func HasCapability[T any](adapter OrchestratorAdapter) bool {
	_, ok := adapter.(T)
	return ok
}
