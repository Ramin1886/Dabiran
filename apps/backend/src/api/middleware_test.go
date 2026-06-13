package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramin1886/git-interactive-history/backend/src/auth"
)

// callWithAuth runs handler through the recorder with the given
// Authorization header value ("" omits the header) and returns the recorder.
func callWithAuth(t *testing.T, handler http.HandlerFunc, header string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestRequireAuthMissingHeader(t *testing.T) {
	handler := RequireAuth(func(w http.ResponseWriter, r *http.Request) { t.Fatal("handler must not run") })
	if rec := callWithAuth(t, handler, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthMalformedHeader(t *testing.T) {
	handler := RequireAuth(func(w http.ResponseWriter, r *http.Request) { t.Fatal("handler must not run") })
	for _, header := range []string{"Basic abc123", "Bearer", "garbage"} {
		if rec := callWithAuth(t, handler, header); rec.Code != http.StatusUnauthorized {
			t.Fatalf("header %q: expected 401, got %d", header, rec.Code)
		}
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	handler := RequireAuth(func(w http.ResponseWriter, r *http.Request) { t.Fatal("handler must not run") })
	if rec := callWithAuth(t, handler, "Bearer not.a.jwt"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthValidTokenInjectsClaims(t *testing.T) {
	token, err := auth.GenerateToken(5, auth.DefaultTeamID, "Team Member")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	called := false
	handler := RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := r.Context().Value(ClaimsContextKey).(*auth.Claims)
		if !ok || claims.UserID != 5 || claims.Role != "Team Member" {
			t.Fatalf("claims not injected correctly: %+v", claims)
		}
		w.WriteHeader(http.StatusOK)
	})
	rec := callWithAuth(t, handler, "Bearer "+token)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("expected handler call with 200, called=%v code=%d", called, rec.Code)
	}
}

func TestRequireRole(t *testing.T) {
	memberToken, _ := auth.GenerateToken(1, auth.DefaultTeamID, "Team Member")
	adminToken, _ := auth.GenerateToken(1, auth.DefaultTeamID, "Admin")

	handler := RequireRole("Admin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if rec := callWithAuth(t, handler, "Bearer "+memberToken); rec.Code != http.StatusForbidden {
		t.Fatalf("member hitting admin route: expected 403, got %d", rec.Code)
	}
	if rec := callWithAuth(t, handler, "Bearer "+adminToken); rec.Code != http.StatusOK {
		t.Fatalf("admin hitting admin route: expected 200, got %d", rec.Code)
	}
	if rec := callWithAuth(t, handler, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous hitting admin route: expected 401, got %d", rec.Code)
	}
}
