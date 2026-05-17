package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type CaddyConfig struct {
	AdminURL      string
	PortPoolStart int
	PortPoolEnd   int
	DomainSuffix  string
}

type PortPool struct {
	mu    sync.Mutex
	start int
	end   int
	used  map[int]bool
}

func NewPortPool(start, end int) *PortPool {
	return &PortPool{
		start: start,
		end:   end,
		used:  make(map[int]bool),
	}
}

func (p *PortPool) Alloc() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port := p.start; port <= p.end; port++ {
		if !p.used[port] {
			p.used[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("port pool exhausted (%d-%d)", p.start, p.end)
}

func (p *PortPool) Free(port int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.used[port] {
		return fmt.Errorf("port %d not allocated", port)
	}
	delete(p.used, port)
	return nil
}

type CaddyAdapter struct {
	mu           sync.Mutex
	client       *http.Client
	adminURL     string
	pool         *PortPool
	domainSuffix string
}

func NewCaddyAdapter(cfg CaddyConfig) *CaddyAdapter {
	adminURL := cfg.AdminURL
	if adminURL == "" {
		adminURL = "http://127.0.0.1:2019"
	}
	start := cfg.PortPoolStart
	if start == 0 {
		start = 9000
	}
	end := cfg.PortPoolEnd
	if end == 0 {
		end = 9999
	}
	suffix := cfg.DomainSuffix
	if suffix == "" {
		suffix = ".mesh.local"
	}

	return &CaddyAdapter{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		adminURL:     adminURL,
		pool:         NewPortPool(start, end),
		domainSuffix: suffix,
	}
}

func (c *CaddyAdapter) Name() string {
	return "caddy"
}

func (c *CaddyAdapter) AllocPort(ctx context.Context, containerPort int) (int, error) {
	return c.pool.Alloc()
}

func (c *CaddyAdapter) FreePort(hostPort int) error {
	return c.pool.Free(hostPort)
}

func (c *CaddyAdapter) AddRoute(ctx context.Context, domain, upstream string, port int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if domain == "" {
		domain = fmt.Sprintf("%s%s", upstream, c.domainSuffix)
	}

	route := map[string]interface{}{
		"@id": domain,
		"match": []map[string]interface{}{
			{"host": []string{domain}},
		},
		"handle": []map[string]interface{}{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]interface{}{
					{"dial": fmt.Sprintf("%s:%d", upstream, port)},
				},
			},
		},
	}

	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshal route: %w", err)
	}

	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%s", c.adminURL, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("caddy admin API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy admin API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *CaddyAdapter) RemoveRoute(ctx context.Context, domain string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%s", c.adminURL, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("caddy admin API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy admin API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *CaddyAdapter) ListRoutes(ctx context.Context) ([]Route, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", c.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("caddy admin API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("caddy admin API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var rawRoutes []caddyRoute
	if err := json.NewDecoder(resp.Body).Decode(&rawRoutes); err != nil {
		return nil, fmt.Errorf("decode routes: %w", err)
	}

	var routes []Route
	for _, r := range rawRoutes {
		domain := ""
		if len(r.Match) > 0 && len(r.Match[0].Host) > 0 {
			domain = r.Match[0].Host[0]
		}
		upstream := ""
		port := 0
		if len(r.Handle) > 0 && len(r.Handle[0].Upstreams) > 0 {
			dial := r.Handle[0].Upstreams[0].Dial
			var host string
			fmt.Sscanf(dial, "%[^:]:%d", &host, &port)
			upstream = host
		}
		routes = append(routes, Route{
			Domain:   domain,
			Upstream: upstream,
			Port:     port,
		})
	}
	return routes, nil
}

type caddyRoute struct {
	ID    string `json:"@id"`
	Match []struct {
		Host []string `json:"host"`
	} `json:"match"`
	Handle []struct {
		Handler   string `json:"handler"`
		Upstreams []struct {
			Dial string `json:"dial"`
		} `json:"upstreams"`
	} `json:"handle"`
}
