package api

import (
	"context"
	"net/http"
	"strings"
)

func BearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			WriteError(w, ErrCodeUnauthorized, "Server not configured with auth token", http.StatusUnauthorized)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			WriteError(w, ErrCodeUnauthorized, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}
		providedToken := strings.TrimPrefix(authHeader, "Bearer ")
		if providedToken != token {
			WriteError(w, ErrCodeUnauthorized, "Invalid bearer token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func JWTOrTokenAuth(cfg RouterConfig, validator *JWTValidator, next http.Handler) http.Handler {
	switch cfg.AuthMode {
	case "jwt":
		if validator != nil {
			return validator.JWTMiddleware(next)
		}
		return BearerAuth(cfg.AuthToken, next)
	case "both":
		return bothModeAuth(cfg, validator, next)
	case "token", "":
		return BearerAuth(cfg.AuthToken, next)
	default:
		return BearerAuth(cfg.AuthToken, next)
	}
}

func bothModeAuth(cfg RouterConfig, validator *JWTValidator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			WriteError(w, ErrCodeUnauthorized, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		if cfg.AuthToken != "" && token == cfg.AuthToken {
			next.ServeHTTP(w, r)
			return
		}

		if validator != nil {
			sub, err := validator.Validate(token)
			if err == nil {
				ctx := context.WithValue(r.Context(), CtxKeyClusterID, sub)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		WriteError(w, ErrCodeUnauthorized, "Invalid bearer token", http.StatusUnauthorized)
	})
}
