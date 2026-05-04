package plugin

import (
	"context"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

type mockPlugin struct{}

func (m *mockPlugin) PluginInfo(ctx context.Context) (PluginMeta, error) {
	return PluginMeta{Name: "mock", Version: "0.0.1"}, nil
}

func (m *mockPlugin) GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error) {
	return &mockOrchestrator{}, nil
}

type mockOrchestrator struct{}

func (o *mockOrchestrator) ScheduleBody(ctx context.Context, spec orchestrator.BodySpec) (orchestrator.Handle, error) {
	return orchestrator.Handle("mock-" + spec.Image), nil
}
func (o *mockOrchestrator) StartBody(ctx context.Context, id orchestrator.Handle) error   { return nil }
func (o *mockOrchestrator) StopBody(ctx context.Context, id orchestrator.Handle) error    { return nil }
func (o *mockOrchestrator) DestroyBody(ctx context.Context, id orchestrator.Handle) error { return nil }
func (o *mockOrchestrator) GetBodyStatus(ctx context.Context, id orchestrator.Handle) (orchestrator.BodyStatus, error) {
	return orchestrator.BodyStatus{State: orchestrator.StateRunning}, nil
}
func (o *mockOrchestrator) Name() string                       { return "mock" }
func (o *mockOrchestrator) IsHealthy(ctx context.Context) bool { return true }

func TestPluginGetOrchestrator(t *testing.T) {
	mp := &mockPlugin{}
	ctx := context.Background()

	orch, err := mp.GetOrchestrator(ctx)
	if err != nil {
		t.Fatalf("GetOrchestrator failed: %v", err)
	}
	if orch == nil {
		t.Fatal("expected orchestrator adapter, got nil")
	}
	if orch.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", orch.Name())
	}
	if !orch.IsHealthy(ctx) {
		t.Error("expected orchestrator to be healthy")
	}
}

func TestPluginInterfaceCompliance(t *testing.T) {
	var _ MeshPlugin = (*mockPlugin)(nil)
}
