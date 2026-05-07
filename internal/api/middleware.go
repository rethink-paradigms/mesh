package api

import (
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
