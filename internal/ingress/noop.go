package ingress

import (
	"context"
	"log/slog"
)

// NoopAdapter is a no-op implementation of IngressAdapter that logs
// intended operations to stderr but performs no actual routing.
// Used when no ingress controller (e.g., Caddy) is configured.
type NoopAdapter struct{}

// NewNoopAdapter creates a new NoopAdapter.
func NewNoopAdapter() *NoopAdapter {
	return &NoopAdapter{}
}

// AddRoute logs the intended route addition to debug log and returns nil.
func (n *NoopAdapter) AddRoute(ctx context.Context, domain, upstream string, port int) error {
	slog.Debug("noop ingress: route add skipped", "domain", domain, "upstream", upstream, "port", port)
	return nil
}

// RemoveRoute logs the intended route removal to debug log and returns nil.
func (n *NoopAdapter) RemoveRoute(ctx context.Context, domain string) error {
	slog.Debug("noop ingress: route remove skipped", "domain", domain)
	return nil
}

// ListRoutes returns an empty slice since no routes are actually configured.
func (n *NoopAdapter) ListRoutes(ctx context.Context) ([]Route, error) {
	return []Route{}, nil
}

// AllocPort logs the intended port allocation to debug log and returns (0, nil).
func (n *NoopAdapter) AllocPort(ctx context.Context, containerPort int) (int, error) {
	slog.Debug("noop ingress: port alloc skipped", "container_port", containerPort)
	return 0, nil
}

// FreePort logs the intended port release to debug log and returns nil.
func (n *NoopAdapter) FreePort(hostPort int) error {
	slog.Debug("noop ingress: port free skipped", "host_port", hostPort)
	return nil
}

// BuildURL returns an empty string since no ingress is configured.
func (n *NoopAdapter) BuildURL(agentName string, hostPort int) string {
	return ""
}

// PublicDomain returns an empty string since no ingress is configured.
func (n *NoopAdapter) PublicDomain() string {
	return ""
}

// PortPoolStats returns zero values since no ingress is configured.
func (n *NoopAdapter) PortPoolStats() (int, int, int, int) {
	return 0, 0, 0, 0
}

// Compile-time check that NoopAdapter implements IngressAdapter.
var _ IngressAdapter = (*NoopAdapter)(nil)
