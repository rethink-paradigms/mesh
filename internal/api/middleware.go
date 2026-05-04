package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

var authWarnOnce sync.Once

func BearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			authWarnOnce.Do(func() {
				fmt.Fprintf(os.Stderr, "WARNING: auth_token not set, API endpoints are unprotected\n")
			})
			next.ServeHTTP(w, r)
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
