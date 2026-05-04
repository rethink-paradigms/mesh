// Package ingress defines the IngressAdapter interface for managing
// HTTP routing to agent bodies, and a no-op stub for when no ingress
// controller (e.g., Caddy) is configured.
package ingress

import "context"

// Route describes a single HTTP route mapping a domain to an upstream service.
type Route struct {
	Domain   string
	Upstream string
	Port     int
}

// IngressAdapter defines the interface for managing HTTP routes
// to body endpoints through an ingress controller.
type IngressAdapter interface {
	// AddRoute creates a route mapping the given domain to upstream:port.
	AddRoute(ctx context.Context, domain string, upstream string, port int) error

	// RemoveRoute deletes the route for the given domain.
	RemoveRoute(ctx context.Context, domain string) error

	// ListRoutes returns all currently configured routes.
	ListRoutes(ctx context.Context) ([]Route, error)
}
