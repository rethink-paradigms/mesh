package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
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
		Kty string `json:"kty"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
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

	jwks := map[string]interface{}{
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

func newTestValidator(t *testing.T, key *rsa.PrivateKey, audience, issuer, ownerID string) (*JWTValidator, *httptest.Server) {
	t.Helper()
	server := testJWKSServer(t, &key.PublicKey)

	jwksURL := server.URL + "/.well-known/jwks.json"
	validator, err := newJWTValidatorWithURL(jwksURL, audience, issuer, ownerID)
	if err != nil {
		server.Close()
		t.Fatalf("create validator: %v", err)
	}
	return validator, server
}

func newJWTValidatorWithURL(jwksURL, audience, issuer, ownerID string) (*JWTValidator, error) {
	if audience == "" {
		return nil, fmt.Errorf("auth0_audience is required")
	}

	k, err := keyfunc.Get(jwksURL, keyfunc.Options{
		RefreshErrorHandler: func(err error) {},
	})
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS from %s: %w", jwksURL, err)
	}

	return &JWTValidator{
		jwks:      k,
		audience:  audience,
		issuer:    issuer,
		ownerID:   ownerID,
		clockSkew: 60 * time.Second,
	}, nil
}

func TestJWTValidToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	sub, err := validator.validate(token)
	if err != nil {
		t.Fatalf("validate valid token: %v", err)
	}
	if sub != "auth0|user123" {
		t.Errorf("sub = %q, want %q", sub, "auth0|user123")
	}
}

func TestJWTExpiredToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	_, err := validator.validate(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error = %q, want containing 'expired'", err.Error())
	}
}

func TestJWTWrongIssuer(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://wrong-issuer.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	_, err := validator.validate(token)
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("error = %q, want containing 'issuer'", err.Error())
	}
}

func TestJWTWrongAudience(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://wrong-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	_, err := validator.validate(token)
	if err == nil {
		t.Fatal("expected error for wrong audience")
	}
	if !strings.Contains(err.Error(), "audience") {
		t.Errorf("error = %q, want containing 'audience'", err.Error())
	}
}

func TestJWTInvalidSignature(t *testing.T) {
	key := generateTestKeys(t)
	wrongKey := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, wrongKey, "test-key-1", claims)

	_, err := validator.validate(token)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
	if !strings.Contains(err.Error(), "signature") && !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("error = %q, want containing 'signature' or 'invalid token'", err.Error())
	}
}

func TestJWTMissingSubClaim(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	_, err := validator.validate(token)
	if err == nil {
		t.Fatal("expected error for missing sub claim")
	}
	if !strings.Contains(err.Error(), "sub") {
		t.Errorf("error = %q, want containing 'sub'", err.Error())
	}
}

func TestJWTWrongOwner(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "auth0|owner123")
	defer server.Close()
	defer validator.Close()

	claims := jwt.MapClaims{
		"sub": "auth0|user123",
		"iss": "https://test-auth0.example.com/",
		"aud": "https://mesh-api",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := generateTestJWT(t, key, "test-key-1", claims)

	_, err := validator.validate(token)
	if err == nil {
		t.Fatal("expected error for wrong owner")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error = %q, want containing 'unauthorized'", err.Error())
	}
}

func TestJWTMiddlewareMissingHeader(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	handler := validator.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var apiErr ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Error.Code != ErrCodeUnauthorized {
		t.Errorf("error code = %q, want %q", apiErr.Error.Code, ErrCodeUnauthorized)
	}
}

func TestJWTMiddlewareMalformedToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	handler := validator.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestJWTMiddlewareValidToken(t *testing.T) {
	key := generateTestKeys(t)
	validator, server := newTestValidator(t, key, "https://mesh-api", "https://test-auth0.example.com/", "")
	defer server.Close()
	defer validator.Close()

	var capturedClusterID string
	handler := validator.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClusterID = ClusterIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if capturedClusterID != "auth0|user123" {
		t.Errorf("cluster_id = %q, want %q", capturedClusterID, "auth0|user123")
	}
}

func TestClusterIDFromContextMissing(t *testing.T) {
	ctx := context.Background()
	id := ClusterIDFromContext(ctx)
	if id != "" {
		t.Errorf("cluster_id = %q, want empty string", id)
	}
}

func TestClusterIDFromContextPresent(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxKeyClusterID, "auth0|user456")
	id := ClusterIDFromContext(ctx)
	if id != "auth0|user456" {
		t.Errorf("cluster_id = %q, want %q", id, "auth0|user456")
	}
}
