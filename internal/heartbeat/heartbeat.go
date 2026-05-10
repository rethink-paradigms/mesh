// Package heartbeat provides a client that periodically sends heartbeat
// pings to a gateway server to report daemon health and status.
package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Client sends periodic heartbeat POSTs to a gateway URL.
type Client struct {
	httpClient   *http.Client
	gatewayURL   string
	authToken    string
	authMode     string
	clusterID    string
	version      string
	tier         string
	orchestrator string
	bodiesCount  func() int
}

// NewClient creates a new heartbeat client with the given configuration.
func NewClient(gatewayURL, authToken, authMode, clusterID, version, tier, orchestrator string, bodiesCount func() int) *Client {
	return &Client{
		gatewayURL:   gatewayURL,
		authToken:    authToken,
		authMode:     authMode,
		clusterID:    clusterID,
		version:      version,
		tier:         tier,
		orchestrator: orchestrator,
		bodiesCount:  bodiesCount,
	}
}

// heartbeatPayload is the JSON shape sent on each heartbeat.
type heartbeatPayload struct {
	ClusterID    string `json:"cluster_id"`
	Status       string `json:"status"`
	BodiesCount  int    `json:"bodies_count"`
	Version      string `json:"version"`
	Tier         string `json:"tier"`
	Orchestrator string `json:"orchestrator"`
}

// Start spawns a goroutine that sends heartbeats at the given interval.
// The goroutine exits when the provided context is cancelled.
// If gatewayURL is empty, Start returns immediately without spawning a goroutine.
// If authMode is "jwt" and authToken is empty, Start logs a warning and returns.
func (c *Client) Start(ctx context.Context, interval time.Duration) {
	if c.gatewayURL == "" {
		return
	}
	if c.authMode == "jwt" && c.authToken == "" {
		log.Printf("[heartbeat] warning: auth_mode is jwt but no auth_token provided; skipping heartbeat")
		return
	}

	go c.loop(ctx, interval)
}

func (c *Client) loop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(ctx); err != nil {
				log.Printf("[heartbeat] warning: %v", err)
			}
		}
	}
}

// sendHeartbeat builds and sends a single heartbeat POST request.
// It uses a 10-second timeout for the HTTP call.
func (c *Client) sendHeartbeat(ctx context.Context) error {
	payload := heartbeatPayload{
		ClusterID:    c.clusterID,
		Status:       "healthy",
		BodiesCount:  c.bodiesCount(),
		Version:      c.version,
		Tier:         c.tier,
		Orchestrator: c.orchestrator,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal heartbeat payload: %w", err)
	}

	url := c.gatewayURL + "/api/internal/heartbeat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create heartbeat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("heartbeat returned %d", resp.StatusCode)
	}

	return nil
}
