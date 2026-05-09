package api

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const CtxKeyClusterID contextKey = "cluster_id"

func ClusterIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(CtxKeyClusterID).(string); ok {
		return id
	}
	return ""
}

type JWTValidator struct {
	jwks      *keyfunc.JWKS
	audience  string
	issuer    string
	ownerID   string
	clockSkew time.Duration
}

func NewJWTValidator(domain, audience, ownerID string) (*JWTValidator, error) {
	if domain == "" {
		return nil, fmt.Errorf("auth0_domain is required")
	}
	if audience == "" {
		return nil, fmt.Errorf("auth0_audience is required")
	}

	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
	issuer := fmt.Sprintf("https://%s/", domain)

	return NewJWTValidatorWithURL(jwksURL, audience, issuer, ownerID)
}

func NewJWTValidatorWithURL(jwksURL, audience, issuer, ownerID string) (*JWTValidator, error) {
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

func (v *JWTValidator) Validate(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, v.jwks.Keyfunc,
		jwt.WithAudience(v.audience),
		jwt.WithIssuer(v.issuer),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithLeeway(v.clockSkew),
	)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "token is expired"):
			return "", fmt.Errorf("token expired")
		case strings.Contains(errMsg, "invalid signature"):
			return "", fmt.Errorf("invalid token signature")
		case strings.Contains(errMsg, "invalid issuer"):
			return "", fmt.Errorf("invalid token issuer")
		case strings.Contains(errMsg, "invalid audience"):
			return "", fmt.Errorf("invalid token audience")
		default:
			return "", fmt.Errorf("invalid token: %s", errMsg)
		}
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", fmt.Errorf("token missing 'sub' claim")
	}

	if v.ownerID != "" {
		if subtle.ConstantTimeCompare([]byte(sub), []byte(v.ownerID)) != 1 {
			return "", fmt.Errorf("unauthorized cluster access")
		}
	}

	return sub, nil
}

func (v *JWTValidator) Close() {
	if v.jwks != nil {
		v.jwks.EndBackground()
	}
}

func (v *JWTValidator) JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			WriteError(w, ErrCodeUnauthorized, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			WriteError(w, ErrCodeUnauthorized, "Empty bearer token", http.StatusUnauthorized)
			return
		}

		sub, err := v.Validate(tokenString)
		if err != nil {
			WriteError(w, ErrCodeUnauthorized, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), CtxKeyClusterID, sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
