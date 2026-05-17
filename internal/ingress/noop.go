package ingress

import (
	"context"
	"fmt"
	"os"
)

// NoopAdapter is a no-op implementation of IngressAdapter that logs
// intended operations to stderr but performs no actual routing.
// Used when no ingress controller (e.g., Caddy) is configured.
type NoopAdapter struct{}

// NewNoopAdapter creates a new NoopAdapter.
func NewNoopAdapter() *NoopAdapter {
	return &NoopAdapter{}
}

// AddRoute logs the intended route addition to stderr and returns nil.
func (n *NoopAdapter) AddRoute(ctx context.Context, domain, upstream string, port int) error {
	fmt.Fprintf(os.Stderr, "ingress: would add route %s → %s:%d (no Caddy configured)\n", domain, upstream, port)
	return nil
}

// RemoveRoute logs the intended route removal to stderr and returns nil.
func (n *NoopAdapter) RemoveRoute(ctx context.Context, domain string) error {
	fmt.Fprintf(os.Stderr, "ingress: would remove route %s (no Caddy configured)\n", domain)
	return nil
}

// ListRoutes returns an empty slice since no routes are actually configured.
func (n *NoopAdapter) ListRoutes(ctx context.Context) ([]Route, error) {
	return []Route{}, nil
}

// AllocPort logs the intended port allocation to stderr and returns (0, nil).
func (n *NoopAdapter) AllocPort(ctx context.Context, containerPort int) (int, error) {
	fmt.Fprintf(os.Stderr, "ingress: would allocate port for container port %d (no Caddy configured)\n", containerPort)
	return 0, nil
}

// FreePort logs the intended port release to stderr and returns nil.
func (n *NoopAdapter) FreePort(hostPort int) error {
	fmt.Fprintf(os.Stderr, "ingress: would free port %d (no Caddy configured)\n", hostPort)
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

// Compile-time check that NoopAdapter implements IngressAdapter.
var _ IngressAdapter = (*NoopAdapter)(nil)
