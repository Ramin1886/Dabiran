package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ramin1886/git-interactive-history/backend/auth"
)

type contextKey string
const ClaimsContextKey contextKey = "claims"

// RequireAuth is a core HTTP middleware resolving Authorization header tokens explicitly enforcing system 1:N access bounds.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "authorization required natively", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, "Bearer ")
		if len(parts) != 2 {
			http.Error(w, "invalid authorization schema formatting", http.StatusUnauthorized)
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

// RequireRole enforces specific RBAC roles securely bypassing context queries resolving authorization levels immediately.
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
		if !ok || claims.Role != role {
			http.Error(w, "insufficient operational mapped roles", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
