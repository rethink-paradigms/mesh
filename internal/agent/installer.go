package agent

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/ingress"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/service"
)

// Installer orchestrates agent installation from a descriptor.
type Installer struct {
	bodyMgr      *body.BodyManager
	ingress      ingress.IngressAdapter
	orchRegistry *orchestrator.Registry
	descriptors  map[string]*Descriptor
	healthPoll   func(ctx context.Context, descriptor *Descriptor, name string, allocatedPorts map[string]int)
}

// InstallResult is returned after a successful agent installation.
type InstallResult struct {
	BodyID         string         `json:"body_id"`
	Name           string         `json:"name"`
	AccessURLs     []string       `json:"access_urls"`
	AllocatedPorts map[string]int `json:"allocated_ports"`
}

// NewInstaller creates a new Installer.
func NewInstaller(bodyMgr *body.BodyManager, ing ingress.IngressAdapter, orchRegistry *orchestrator.Registry, descriptors map[string]*Descriptor) *Installer {
	i := &Installer{
		bodyMgr:      bodyMgr,
		ingress:      ing,
		orchRegistry: orchRegistry,
		descriptors:  descriptors,
	}
	i.healthPoll = i.defaultPollHealth
	return i
}

// Install installs an agent from a descriptor.
// Flow: resolve descriptor → validate env → allocate ports → create body → routes → health check.
func (i *Installer) Install(ctx context.Context, agentType, name string, env map[string]string, configFiles map[string]string, descriptorYAML string) (*InstallResult, error) {
	// 1. Resolve descriptor
	var descriptor *Descriptor
	if descriptorYAML != "" {
		var err error
		descriptor, err = ParseDescriptor([]byte(descriptorYAML))
		if err != nil {
			return nil, err
		}
	} else {
		var ok bool
		descriptor, ok = i.descriptors[agentType]
		if !ok {
			return nil, &service.NotFoundError{ID: agentType}
		}
	}

	// 2. Validate required env vars
	if err := ValidateEnv(descriptor, env); err != nil {
		return nil, &service.ValidationError{Field: "env", Message: err.Error()}
	}

	if i.bodyMgr != nil {
		bodies, err := i.bodyMgr.List(ctx)
		if err == nil {
			for _, b := range bodies {
				if b.Name == name {
					return nil, &service.ConflictError{State: "exists", Required: "unique name"}
				}
			}
		}
	}

	// 4. Merge env defaults (optional vars from descriptor)
	mergedEnv := make(map[string]string)
	for _, key := range descriptor.Env.Optional {
		if val, ok := env[key]; ok {
			mergedEnv[key] = val
		}
	}
	for k, v := range env {
		mergedEnv[k] = v
	}

	// 5. Allocate ports from ingress pool BEFORE building body spec.
	// This ensures HostPort is set on BodyPort so Docker binds to the correct port,
	// not a random dynamic port.
	allocatedPorts := make(map[string]int)
	if i.ingress != nil {
		for _, p := range descriptor.Ports {
			if !p.Expose {
				continue
			}
			hostPort, err := i.ingress.AllocPort(ctx, p.ContainerPort)
			if err != nil {
				// Best effort: log and continue
				continue
			}
			allocatedPorts[p.Name] = hostPort
		}
	}

	// 6. Build BodySpec from descriptor (with HostPort set from allocation)
	ports := make([]orchestrator.BodyPort, len(descriptor.Ports))
	for i, p := range descriptor.Ports {
		ports[i] = orchestrator.BodyPort{
			Name:          p.Name,
			ContainerPort: p.ContainerPort,
			HostPort:      allocatedPorts[p.Name],
			Protocol:      p.Protocol,
			Expose:        p.Expose,
		}
	}

	// Merge config files from the request (runtime-generated, e.g. Agent Vault proxy)
	// with static files from the descriptor manifest. Request files take precedence.
	mergedFiles := make(map[string]string)
	for k, v := range descriptor.ConfigFiles {
		mergedFiles[k] = v
	}
	for k, v := range configFiles {
		mergedFiles[k] = v
	}

	spec := orchestrator.BodySpec{
		Image:     descriptor.Image,
		Workdir:   "/workspace",
		Env:       mergedEnv,
		Files:     mergedFiles,
		Cmd:       descriptor.Command,
		MemoryMB:  descriptor.Resources.MemoryMB,
		CPUShares: descriptor.Resources.CPUShares,
		Ports:     ports,
	}

	// 7. Create body (Docker creates container with correct HostPort now)
	b, err := i.bodyMgr.Create(ctx, name, spec)
	if err != nil {
		return nil, fmt.Errorf("create body: %w", err)
	}

	// 8. Build access URLs and create ingress routes from allocated ports
	var accessURLs []string
	if i.ingress != nil {
		for _, p := range descriptor.Ports {
			if !p.Expose {
				continue
			}
			hostPort, ok := allocatedPorts[p.Name]
			if !ok {
				continue
			}
			url := i.ingress.BuildURL(name, hostPort)
			if url != "" {
				accessURLs = append(accessURLs, url)
			}
			if i.ingress.PublicDomain() != "" {
				domain := fmt.Sprintf("%s.%s", name, i.ingress.PublicDomain())
				_ = i.ingress.AddRoute(ctx, domain, "127.0.0.1", hostPort)
			}
		}
	}

	if descriptor.HealthCheck != nil && i.healthPoll != nil {
		go i.healthPoll(context.Background(), descriptor, name, allocatedPorts)
	}

	return &InstallResult{
		BodyID:         b.ID,
		Name:           name,
		AccessURLs:     accessURLs,
		AllocatedPorts: allocatedPorts,
	}, nil
}

// IngressAdapter returns the ingress adapter used by this installer.
func (i *Installer) IngressAdapter() ingress.IngressAdapter {
	return i.ingress
}

// Uninstall uninstalls an agent by name, stopping it first if running, then destroying its body.
func (i *Installer) Uninstall(ctx context.Context, agentName string) error {
	if i.bodyMgr == nil {
		return fmt.Errorf("body manager not configured")
	}
	bodies, err := i.bodyMgr.List(ctx)
	if err != nil {
		return fmt.Errorf("list bodies: %w", err)
	}
	for _, b := range bodies {
		if b.Name == agentName {
			// Stop first if running — Destroy requires Stopped/Error state
			if b.State == orchestrator.StateRunning || b.State == orchestrator.StateStarting {
				if stopErr := i.bodyMgr.Stop(ctx, b.ID, orchestrator.StopOpts{}); stopErr != nil {
					return fmt.Errorf("stop body before destroy: %w", stopErr)
				}
			}
			return i.bodyMgr.Destroy(ctx, b.ID)
		}
	}
	return &service.NotFoundError{ID: agentName}
}

func (i *Installer) defaultPollHealth(ctx context.Context, descriptor *Descriptor, name string, allocatedPorts map[string]int) {
	if descriptor.HealthCheck.Type != "http" {
		return
	}

	// Find the allocated host port for the health check container port
	var healthHostPort int
	for _, p := range descriptor.Ports {
		if fmt.Sprintf("%d", p.ContainerPort) == descriptor.HealthCheck.Port && p.Expose {
			healthHostPort = allocatedPorts[p.Name]
			break
		}
	}
	if healthHostPort == 0 {
		return
	}

	url := i.ingress.BuildURL(name, healthHostPort) + descriptor.HealthCheck.Path

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				continue
			}
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close() //nolint:errcheck
				if resp.StatusCode == http.StatusOK {
					return
				}
			}
		}
	}
}
