package mcp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rethink-paradigms/mesh/internal/api"
	"github.com/rethink-paradigms/mesh/internal/service"
	"github.com/rethink-paradigms/mesh/internal/store"
)

func generateTestKeys(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func testJWKSServer(t *testing.T, pubKey *rsa.PublicKey) *httptest.Server {
	t.Helper()

	pubKeyDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	kid := "test-key-1"
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: pubKeyDER}
	pemBytes := pem.EncodeToMemory(block)

	type jwkKey struct {
		Kty string   `json:"kty"`
		Use string   `json:"use"`
		Alg string   `json:"alg"`
		Kid string   `json:"kid"`
		N   string   `json:"n"`
		E   string   `json:"e"`
		X5c []string `json:"x5c,omitempty"`
	}

	jwk := jwkKey{
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		Kid: kid,
		N:   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
		X5c: []string{base64.StdEncoding.EncodeToString(pemBytes)},
	}

	jwks := map[string]any{
		"keys": []jwkKey{jwk},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})

	return httptest.NewServer(mux)
}

func generateTestJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return signed
}

func newTestValidator(t *testing.T, key *rsa.PrivateKey, audience, issuer, ownerID string) (*api.JWTValidator, *httptest.Server) {
	t.Helper()
	server := testJWKSServer(t, &key.PublicKey)

	jwksURL := server.URL + "/.well-known/jwks.json"
	validator, err := api.NewJWTValidatorWithURL(jwksURL, audience, issuer, ownerID)
	if err != nil {
		server.Close()
		t.Fatalf("create validator: %v", err)
	}
	return validator, server
}

func setupAuthTest(t *testing.T) (*Server, *api.JWTValidator, string, func()) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bm := testBodyManager(t, s)

	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "test-aud", "test-iss", "")

	claims := jwt.MapClaims{
		"sub": "auth0|test-user",
		"iss": "test-iss",
		"aud": "test-aud",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	srv := NewWithIO(s, strings.NewReader(""), &strings.Builder{})
	validator2, err := api.NewJWTValidatorWithURL(server.URL+"/.well-known/jwks.json", "test-aud", "test-iss", "")
	if err != nil {
		server.Close()
		validator.Close()
		s.Close()
		t.Fatalf("create validator: %v", err)
	}
	srv.authValidator = validator2
	srv.authEnabled = true
	srv.SetBodyManager(bm)
	srv.SetBodyService(service.NewBodyService(bm, s, nil))

	cleanup := func() {
		server.Close()
		validator.Close()
		validator2.Close()
		s.Close()
	}

	return srv, validator, token, cleanup
}

func TestMCPAuth_NoAuthToken_Rejected(t *testing.T) {
	srv, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != -32000 {
		t.Errorf("error code = %d, want %d", resp.Error.Code, -32000)
	}
	if !strings.Contains(resp.Error.Message, "authentication required") {
		t.Errorf("error message = %q, want containing 'authentication required'", resp.Error.Message)
	}
}

func TestMCPAuth_ValidAuthToken_Accepted(t *testing.T) {
	srv, _, token, cleanup := setupAuthTest(t)
	defer cleanup()

	os.Setenv("MESH_JWT_TOKEN", token)
	defer os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
}

func TestMCPAuth_ExpiredAuthToken_Rejected(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	bm := testBodyManager(t, s)

	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "test-aud", "test-iss", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|test-user",
		"iss": "test-iss",
		"aud": "test-aud",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	srv := NewWithIO(s, strings.NewReader(""), &strings.Builder{})
	validator2, err := api.NewJWTValidatorWithURL(server.URL+"/.well-known/jwks.json", "test-aud", "test-iss", "")
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}
	defer validator2.Close()
	srv.authValidator = validator2
	srv.authEnabled = true
	srv.SetBodyManager(bm)
	srv.SetBodyService(service.NewBodyService(bm, s, nil))

	os.Setenv("MESH_JWT_TOKEN", token)
	defer os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if resp.Error.Code != -32000 {
		t.Errorf("error code = %d, want %d", resp.Error.Code, -32000)
	}
	if !strings.Contains(resp.Error.Message, "expired") {
		t.Errorf("error message = %q, want containing 'expired'", resp.Error.Message)
	}
}

func TestMCPAuth_InvalidAuthToken_Rejected(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	bm := testBodyManager(t, s)

	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "test-aud", "test-iss", "")
	defer server.Close()
	defer validator.Close()

	srv := NewWithIO(s, strings.NewReader(""), &strings.Builder{})
	validator2, err := api.NewJWTValidatorWithURL(server.URL+"/.well-known/jwks.json", "test-aud", "test-iss", "")
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}
	defer validator2.Close()
	srv.authValidator = validator2
	srv.authEnabled = true
	srv.SetBodyManager(bm)
	srv.SetBodyService(service.NewBodyService(bm, s, nil))

	os.Setenv("MESH_JWT_TOKEN", "invalid-token")
	defer os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
	if resp.Error.Code != -32000 {
		t.Errorf("error code = %d, want %d", resp.Error.Code, -32000)
	}
}

func TestMCPAuth_AuthDisabled_NoTokenRequired(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	bm := testBodyManager(t, s)
	srv := NewWithIO(s, strings.NewReader(""), &strings.Builder{})
	srv.SetBodyManager(bm)
	srv.SetBodyService(service.NewBodyService(bm, s, nil))

	os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error when auth disabled: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
}

func TestMCPAuthGating_NoToken(t *testing.T) {
	srv, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != -32000 {
		t.Errorf("error code = %d, want %d", resp.Error.Code, -32000)
	}
	if !strings.Contains(resp.Error.Message, "authentication required") {
		t.Errorf("error message = %q, want containing 'authentication required'", resp.Error.Message)
	}
}

func TestMCPAuthGating_ValidToken(t *testing.T) {
	srv, _, token, cleanup := setupAuthTest(t)
	defer cleanup()

	os.Setenv("MESH_JWT_TOKEN", token)
	defer os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
}

func TestMCPAuthGating_AllToolsRequireAuth(t *testing.T) {
	srv, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	os.Unsetenv("MESH_JWT_TOKEN")

	tools := []string{"ping", "list_bodies", "get_body", "get_snapshot", "execute_command", "create_body", "delete_body", "migrate_body", "start_body", "stop_body", "create_snapshot", "list_snapshots", "restore_body", "get_body_logs", "get_body_status", "list_plugins", "plugin_health"}

	for _, toolName := range tools {
		var buf strings.Builder
		srv.writer = &buf

		req := Request{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params:  json.RawMessage(`{"name":"` + toolName + `","arguments":{}}`),
		}
		srv.handle(context.Background(), req)

		var resp Response
		if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
			t.Fatalf("tool %s: unmarshal response: %v", toolName, err)
		}
		if resp.Error == nil {
			t.Fatalf("tool %s: expected error, got nil", toolName)
		}
		if resp.Error.Code != -32000 {
			t.Fatalf("tool %s: error code = %d, want %d", toolName, resp.Error.Code, -32000)
		}
		if !strings.Contains(resp.Error.Message, "authentication required") {
			t.Fatalf("tool %s: error message = %q, want 'authentication required'", toolName, resp.Error.Message)
		}
	}
}

func TestMCPAuthGating_PingRequiresAuth(t *testing.T) {
	srv, _, _, cleanup := setupAuthTest(t)
	defer cleanup()

	os.Unsetenv("MESH_JWT_TOKEN")

	var buf strings.Builder
	srv.writer = &buf

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"ping","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp Response
	if err := json.Unmarshal([]byte(buf.String()), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != -32000 {
		t.Errorf("error code = %d, want %d", resp.Error.Code, -32000)
	}
	if !strings.Contains(resp.Error.Message, "authentication required") {
		t.Errorf("error message = %q, want containing 'authentication required'", resp.Error.Message)
	}
}

func TestMCP_AuthCachesClusterID(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	bm := testBodyManager(t, s)

	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "test-aud", "test-iss", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|test-user",
		"iss": "test-iss",
		"aud": "test-aud",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	srv := NewWithIO(s, strings.NewReader(""), &strings.Builder{})
	validator2, err := api.NewJWTValidatorWithURL(server.URL+"/.well-known/jwks.json", "test-aud", "test-iss", "")
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}
	srv.authValidator = validator2
	srv.authEnabled = true
	srv.SetBodyManager(bm)
	srv.SetBodyService(service.NewBodyService(bm, s, nil))

	os.Setenv("MESH_JWT_TOKEN", token)
	defer os.Unsetenv("MESH_JWT_TOKEN")

	var buf1 strings.Builder
	srv.writer = &buf1

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req)

	var resp1 Response
	if err := json.Unmarshal([]byte(buf1.String()), &resp1); err != nil {
		t.Fatalf("unmarshal response 1: %v", err)
	}
	if resp1.Error != nil {
		t.Fatalf("unexpected error on first call: %v", resp1.Error)
	}

	if srv.clusterID != "auth0|test-user" {
		t.Errorf("clusterID = %q, want %q", srv.clusterID, "auth0|test-user")
	}

	var buf2 strings.Builder
	srv.writer = &buf2

	req2 := Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_bodies","arguments":{}}`),
	}
	srv.handle(context.Background(), req2)

	var resp2 Response
	if err := json.Unmarshal([]byte(buf2.String()), &resp2); err != nil {
		t.Fatalf("unmarshal response 2: %v", err)
	}
	if resp2.Error != nil {
		t.Fatalf("unexpected error on second call: %v", resp2.Error)
	}
}
