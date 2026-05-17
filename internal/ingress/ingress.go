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

	// AllocPort allocates a host port and maps it to the given container port.
	// Returns the allocated host port.
	AllocPort(ctx context.Context, containerPort int) (int, error)

	// FreePort releases a previously allocated host port.
	FreePort(hostPort int) error

	// BuildURL returns the external access URL for an agent based on ingress
	// configuration. When public_domain is set, returns https://{agentName}.{publicDomain}.
	// When public_domain is empty, returns http://127.0.0.1:{hostPort} for VM-local access.
	BuildURL(agentName string, hostPort int) string

	// PublicDomain returns the configured public domain, or empty string if
	// direct port mode is active.
	PublicDomain() string
}
