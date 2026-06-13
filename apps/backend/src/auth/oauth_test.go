package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// stateCookie extracts the oauth state cookie from a recorded response.
func stateCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == stateCookieName {
			return c
		}
	}
	return nil
}

func TestHandleLoginSetsStateCookieAndRedirects(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleLogin(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login", nil))

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rec.Code)
	}
	cookie := stateCookie(t, rec)
	if cookie == nil || cookie.Value == "" {
		t.Fatal("state cookie not set")
	}
	if !cookie.HttpOnly {
		t.Fatal("state cookie must be HttpOnly")
	}

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("invalid redirect location: %v", err)
	}
	if got := loc.Query().Get("state"); got != cookie.Value {
		t.Fatalf("redirect state %q does not match cookie %q", got, cookie.Value)
	}
}

func TestHandleLoginStateIsRandom(t *testing.T) {
	values := map[string]bool{}
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		HandleLogin(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
		c := stateCookie(t, rec)
		if c == nil {
			t.Fatal("state cookie not set")
		}
		if values[c.Value] {
			t.Fatal("state value repeated across logins")
		}
		values[c.Value] = true
	}
}

func TestHandleCallbackRejectsMismatchedState(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/callback?state=forged&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "genuine"})
	rec := httptest.NewRecorder()
	HandleCallback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for mismatched state, got %d", rec.Code)
	}
}

func TestHandleCallbackRejectsMissingStateCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/callback?state=whatever&code=abc", nil)
	rec := httptest.NewRecorder()
	HandleCallback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing state cookie, got %d", rec.Code)
	}
}
