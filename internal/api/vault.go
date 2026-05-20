package api

import (
	"io"
	"net/http"
	"strings"
)

// Agent Vault runs on the same VM, bound to localhost.
// The daemon proxies agent-bodies requests to it so that:
//   - Agent Vault's management API never needs to be network-exposed
//   - The daemon's existing auth (JWT/token) is reused
//   - agent-bodies doesn't need a separate Agent Vault token
const agentVaultBaseURL = "http://127.0.0.1:14321"

// handleVaultProxy forwards requests to the local Agent Vault API.
// agent-bodies calls POST /api/v1/vault/{path} on the daemon,
// which proxies to http://127.0.0.1:14321/{path}.
//
// All HTTP methods pass through (GET, POST, DELETE, etc.) so the
// full Agent Vault REST API is accessible.
func (h *Handler) handleVaultProxy(w http.ResponseWriter, r *http.Request) {
	// Strip /api/v1/vault prefix to get the Agent Vault API path
	targetPath := strings.TrimPrefix(r.URL.Path, "/api/v1/vault")
	if targetPath == "" || targetPath[0] != '/' {
		targetPath = "/" + targetPath
	}

	targetURL := agentVaultBaseURL + targetPath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to create proxy request"}`, http.StatusInternalServerError)
		return
	}

	// Copy original headers (skip hop-by-hop and daemon auth)
	for key, vals := range r.Header {
		for _, v := range vals {
			proxyReq.Header.Add(key, v)
		}
	}
	proxyReq.Header.Del("Authorization")

	// Inject the Agent Vault admin token so agent-bodies doesn't
	// need to know it. The token is set in the daemon config
	// (from Infisical) during VM provisioning.
	if h.cfg.AgentVaultToken != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+h.cfg.AgentVaultToken)
	}

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		http.Error(w, `{"error":"vault proxy: `+err.Error()+`"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers and body
	for key, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// vaultProxyHandler wraps handleVaultProxy as an http.HandlerFunc.
// It catches all HTTP methods (GET, POST, DELETE, etc.).
func vaultProxyHandler(h *Handler) http.HandlerFunc {
	return h.handleVaultProxy
}
