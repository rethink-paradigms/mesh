package orchestrator

import (
	"context"
	"io"
	"testing"
)

type fullMockAdapter struct{}

func (f *fullMockAdapter) ScheduleBody(ctx context.Context, spec BodySpec) (Handle, error) {
	return "", nil
}
func (f *fullMockAdapter) StartBody(ctx context.Context, id Handle) error   { return nil }
func (f *fullMockAdapter) StopBody(ctx context.Context, id Handle) error    { return nil }
func (f *fullMockAdapter) DestroyBody(ctx context.Context, id Handle) error { return nil }
func (f *fullMockAdapter) GetBodyStatus(ctx context.Context, id Handle) (BodyStatus, error) {
	return BodyStatus{}, nil
}
func (f *fullMockAdapter) Name() string                       { return "full" }
func (f *fullMockAdapter) IsHealthy(ctx context.Context) bool { return true }

func (f *fullMockAdapter) ExportFilesystem(ctx context.Context, id Handle) (io.ReadCloser, error) {
	return nil, nil
}
func (f *fullMockAdapter) ImportFilesystem(ctx context.Context, id Handle, tarball io.Reader) error {
	return nil
}
func (f *fullMockAdapter) Inspect(ctx context.Context, id Handle) (ContainerMetadata, error) {
	return ContainerMetadata{}, nil
}
func (f *fullMockAdapter) Exec(ctx context.Context, id Handle, cmd []string) (ExecResult, error) {
	return ExecResult{}, nil
}

type minimalMockAdapter struct{}

func (m *minimalMockAdapter) ScheduleBody(ctx context.Context, spec BodySpec) (Handle, error) {
	return "", nil
}
func (m *minimalMockAdapter) StartBody(ctx context.Context, id Handle) error   { return nil }
func (m *minimalMockAdapter) StopBody(ctx context.Context, id Handle) error    { return nil }
func (m *minimalMockAdapter) DestroyBody(ctx context.Context, id Handle) error { return nil }
func (m *minimalMockAdapter) GetBodyStatus(ctx context.Context, id Handle) (BodyStatus, error) {
	return BodyStatus{}, nil
}
func (m *minimalMockAdapter) Name() string                       { return "minimal" }
func (m *minimalMockAdapter) IsHealthy(ctx context.Context) bool { return true }

func TestExtensionTypeAssertion(t *testing.T) {
	var full OrchestratorAdapter = &fullMockAdapter{}
	var minimal OrchestratorAdapter = &minimalMockAdapter{}

	if _, ok := full.(Exporter); !ok {
		t.Error("fullMockAdapter should implement Exporter")
	}
	if _, ok := full.(Importer); !ok {
		t.Error("fullMockAdapter should implement Importer")
	}
	if _, ok := full.(Inspector); !ok {
		t.Error("fullMockAdapter should implement Inspector")
	}
	if _, ok := full.(Executor); !ok {
		t.Error("fullMockAdapter should implement Executor")
	}

	if _, ok := minimal.(Exporter); ok {
		t.Error("minimalMockAdapter should NOT implement Exporter")
	}
	if _, ok := minimal.(Importer); ok {
		t.Error("minimalMockAdapter should NOT implement Importer")
	}
	if _, ok := minimal.(Inspector); ok {
		t.Error("minimalMockAdapter should NOT implement Inspector")
	}
	if _, ok := minimal.(Executor); ok {
		t.Error("minimalMockAdapter should NOT implement Executor")
	}
}

func TestHasCapability(t *testing.T) {
	full := &fullMockAdapter{}
	minimal := &minimalMockAdapter{}

	if !HasCapability[Exporter](full) {
		t.Error("HasCapability[Exporter](fullAdapter) should be true")
	}
	if HasCapability[Exporter](minimal) {
		t.Error("HasCapability[Exporter](minimalAdapter) should be false")
	}
	if !HasCapability[Executor](full) {
		t.Error("HasCapability[Executor](fullAdapter) should be true")
	}
	if HasCapability[Executor](minimal) {
		t.Error("HasCapability[Executor](minimalAdapter) should be false")
	}
	if !HasCapability[Importer](full) {
		t.Error("HasCapability[Importer](fullAdapter) should be true")
	}
	if HasCapability[Importer](minimal) {
		t.Error("HasCapability[Importer](minimalAdapter) should be false")
	}
	if !HasCapability[Inspector](full) {
		t.Error("HasCapability[Inspector](fullAdapter) should be true")
	}
	if HasCapability[Inspector](minimal) {
		t.Error("HasCapability[Inspector](minimalAdapter) should be false")
	}
}

func TestImporterInterface(t *testing.T) {
	var _ Importer = (*fullMockAdapter)(nil)
}

func TestInspectorInterface(t *testing.T) {
	var _ Inspector = (*fullMockAdapter)(nil)
}

func TestExporterInterface(t *testing.T) {
	var _ Exporter = (*fullMockAdapter)(nil)
}

func TestExecutorInterface(t *testing.T) {
	var _ Executor = (*fullMockAdapter)(nil)
}
