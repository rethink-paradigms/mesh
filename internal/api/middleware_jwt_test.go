package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTOrTokenAuth_TokenMode_ValidToken(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-secret", AuthMode: "token"}
	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("token mode valid: status = %d, want 200", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_TokenMode_InvalidToken(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-secret", AuthMode: "token"}
	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("token mode invalid: status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_TokenMode_MissingHeader(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-secret", AuthMode: "token"}
	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("token mode missing: status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_DefaultMode(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-secret", AuthMode: ""}
	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("default mode: status = %d, want 200", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_UnknownMode(t *testing.T) {
	cfg := RouterConfig{AuthToken: "test-secret", AuthMode: "unknown"}
	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unknown mode: status = %d, want 200", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_JWTMode_ValidToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("jwt mode valid: status = %d, want 200", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_JWTMode_InvalidToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer garbage-token")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("jwt mode invalid: status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_JWTMode_MissingHeader(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("jwt mode missing: status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_JWTMode_FallbackToToken(t *testing.T) {
	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "jwt",
		Auth0Domain:    "",
		Auth0Audience:  "",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("jwt fallback to token: status = %d, want 200", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_BothMode_ValidToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "both",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("both mode token: status = %d, want 200", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_BothMode_ValidJWT(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "both",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	var capturedClusterID string
	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClusterID = ClusterIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("both mode jwt: status = %d, want 200", resp.StatusCode)
	}
	if capturedClusterID != "auth0|user123" {
		t.Errorf("both mode jwt cluster_id = %q, want %q", capturedClusterID, "auth0|user123")
	}
}

func TestJWTOrTokenAuth_BothMode_InvalidBoth(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "both",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("both mode invalid: status = %d, want 401", resp.StatusCode)
	}

	var apiErr ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Error.Code != ErrCodeUnauthorized {
		t.Errorf("both mode invalid code = %q, want %q", apiErr.Error.Code, ErrCodeUnauthorized)
	}
}

func TestJWTOrTokenAuth_BothMode_MissingHeader(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "both",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("both mode missing: status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_BothMode_NoValidator(t *testing.T) {
	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "both",
		Auth0Domain:    "",
		Auth0Audience:  "",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("both mode no validator token: status = %d, want 200", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_BothMode_NoValidator_InvalidToken(t *testing.T) {
	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "both",
		Auth0Domain:    "",
		Auth0Audience:  "",
		ClusterOwnerID: "",
	}

	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("both mode no validator invalid: status = %d, want 401", resp.StatusCode)
	}
}

func TestJWTOrTokenAuth_JWTMode_ContextInjection(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	cfg := RouterConfig{
		AuthMode:       "jwt",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	var capturedClusterID string
	handler := JWTOrTokenAuth(cfg, validator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClusterID = ClusterIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user456",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("jwt context: status = %d, want 200", resp.StatusCode)
	}
	if capturedClusterID != "auth0|user456" {
		t.Errorf("jwt context cluster_id = %q, want %q", capturedClusterID, "auth0|user456")
	}
}

func TestJWTOrTokenAuth_BothMode_TokenAuthNoContext(t *testing.T) {
	cfg := RouterConfig{
		AuthToken:      "test-secret",
		AuthMode:       "both",
		Auth0Domain:    "test-auth0.example.com",
		Auth0Audience:  "https://mesh-api",
		ClusterOwnerID: "",
	}

	var capturedClusterID string
	handler := JWTOrTokenAuth(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClusterID = ClusterIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("both token no context: status = %d, want 200", resp.StatusCode)
	}
	if capturedClusterID != "" {
		t.Errorf("both token no context cluster_id = %q, want empty", capturedClusterID)
	}
}
