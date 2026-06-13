package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
)

// contextKey is a private type so context values set here cannot collide
// with keys from other packages.
type contextKey string

// ClaimsContextKey locates the validated *auth.Claims in a request context.
const ClaimsContextKey contextKey = "claims"

// RequireAuth wraps next, rejecting requests without a valid "Bearer <jwt>"
// Authorization header (401) and injecting the validated claims into the
// request context under ClaimsContextKey.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, "Bearer ")
		if len(parts) != 2 {
			http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// RequireRole wraps next with RequireAuth and additionally rejects callers
// whose JWT role claim differs from role (403).
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
		if !ok || claims.Role != role {
			http.Error(w, "insufficient role", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
