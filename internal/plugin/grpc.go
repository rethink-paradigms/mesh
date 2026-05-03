package plugin

import (
	"context"
	"fmt"
	"io"

	"github.com/hashicorp/go-plugin"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"google.golang.org/grpc"
)

const PluginName = "mesh-plugin"

var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "MESH_PLUGIN",
	MagicCookieValue: "mesh-plugin-2026",
}

var PluginMap = map[string]plugin.Plugin{
	PluginName: &MeshPluginGRPC{},
}

type MeshPluginGRPC struct {
	plugin.Plugin
	Impl MeshPlugin
}

func (p *MeshPluginGRPC) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	RegisterMeshPluginServer(s, &meshPluginServer{impl: p.Impl})
	return nil
}

func (p *MeshPluginGRPC) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &meshPluginRPCClient{grpcClient: NewMeshPluginClient(c)}, nil
}

type meshPluginServer struct {
	UnimplementedMeshPluginServer
	impl MeshPlugin
}

func (s *meshPluginServer) PluginInfo(ctx context.Context, req *PluginMetaRequest) (*PluginMetaResponse, error) {
	meta, err := s.impl.PluginInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &PluginMetaResponse{
		Name:        meta.Name,
		Version:     meta.Version,
		Description: meta.Description,
		Author:      meta.Author,
	}, nil
}

func (s *meshPluginServer) HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	adapter, err := s.impl.GetAdapter(ctx)
	if err != nil {
		return &HealthCheckResponse{Healthy: false, Message: err.Error()}, nil
	}
	healthy := adapter.IsHealthy(ctx)
	msg := "ok"
	if !healthy {
		msg = "adapter not healthy"
	}
	return &HealthCheckResponse{Healthy: healthy, Message: msg}, nil
}

func (s *meshPluginServer) GetAdapter(ctx context.Context, req *GetAdapterRequest) (*GetAdapterResponse, error) {
	adapter, err := s.impl.GetAdapter(ctx)
	if err != nil {
		return nil, err
	}
	caps := adapter.Capabilities()
	return &GetAdapterResponse{
		SubstrateName:   adapter.SubstrateName(),
		SupportsExport:  caps.ExportFilesystem,
		SupportsImport:  caps.ImportFilesystem,
		SupportsInspect: caps.Inspect,
	}, nil
}

type meshPluginRPCClient struct {
	grpcClient MeshPluginClient
}

func (c *meshPluginRPCClient) PluginInfo(ctx context.Context) (PluginMeta, error) {
	resp, err := c.grpcClient.PluginInfo(ctx, &PluginMetaRequest{})
	if err != nil {
		return PluginMeta{}, err
	}
	return PluginMeta{
		Name:        resp.Name,
		Version:     resp.Version,
		Description: resp.Description,
		Author:      resp.Author,
	}, nil
}

func (c *meshPluginRPCClient) GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error) {
	resp, err := c.grpcClient.GetAdapter(ctx, &GetAdapterRequest{})
	if err != nil {
		return nil, err
	}
	return &grpcAdapterProxy{
		substrateName: resp.SubstrateName,
		caps: adapter.AdapterCapabilities{
			ExportFilesystem: resp.SupportsExport,
			ImportFilesystem: resp.SupportsImport,
			Inspect:          resp.SupportsInspect,
		},
	}, nil
}

func (c *meshPluginRPCClient) GetOrchestrator(ctx context.Context) (orchestrator.OrchestratorAdapter, error) {
	return nil, fmt.Errorf("GetOrchestrator not available over gRPC; use GetAdapter")
}

type grpcAdapterProxy struct {
	substrateName string
	caps          adapter.AdapterCapabilities
}

func (a *grpcAdapterProxy) Create(ctx context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	return "", fmt.Errorf("grpcAdapterProxy.Create not implemented")
}
func (a *grpcAdapterProxy) Start(ctx context.Context, id adapter.Handle) error {
	return fmt.Errorf("grpcAdapterProxy.Start not implemented")
}
func (a *grpcAdapterProxy) Stop(ctx context.Context, id adapter.Handle, opts adapter.StopOpts) error {
	return fmt.Errorf("grpcAdapterProxy.Stop not implemented")
}
func (a *grpcAdapterProxy) Destroy(ctx context.Context, id adapter.Handle) error {
	return fmt.Errorf("grpcAdapterProxy.Destroy not implemented")
}
func (a *grpcAdapterProxy) GetStatus(ctx context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{}, fmt.Errorf("grpcAdapterProxy.GetStatus not implemented")
}
func (a *grpcAdapterProxy) Exec(ctx context.Context, id adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{}, fmt.Errorf("grpcAdapterProxy.Exec not implemented")
}
func (a *grpcAdapterProxy) ExportFilesystem(ctx context.Context, id adapter.Handle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("grpcAdapterProxy.ExportFilesystem not implemented")
}
func (a *grpcAdapterProxy) ImportFilesystem(ctx context.Context, id adapter.Handle, tarball io.Reader, opts adapter.ImportOpts) error {
	return fmt.Errorf("grpcAdapterProxy.ImportFilesystem not implemented")
}
func (a *grpcAdapterProxy) Inspect(ctx context.Context, id adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, fmt.Errorf("grpcAdapterProxy.Inspect not implemented")
}
func (a *grpcAdapterProxy) Capabilities() adapter.AdapterCapabilities {
	return a.caps
}
func (a *grpcAdapterProxy) SubstrateName() string {
	return a.substrateName
}
func (a *grpcAdapterProxy) IsHealthy(ctx context.Context) bool {
	return true
}
