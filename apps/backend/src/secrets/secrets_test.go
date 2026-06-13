package secrets

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testKey returns a deterministic 32-byte AES-256 key for tests.
func testKey(b byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = b
	}
	return key
}

// fakeVault returns an httptest server that serves a KV v2 response with the
// supplied repo_cred_key value at the given path. An empty wantPath matches any
// request path.
func fakeVault(t *testing.T, wantPath, repoCredKeyVal string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") == "" {
			http.Error(w, "missing token", http.StatusForbidden)
			return
		}
		if wantPath != "" && r.URL.Path != wantPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp := kvV2Response{}
		resp.Data.Data = map[string]string{repoCredKeyName: repoCredKeyVal}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestResolveMasterKeyFromVault(t *testing.T) {
	want := testKey(0x5a)
	srv := fakeVault(t, "/v1/"+defaultKVPath, base64.StdEncoding.EncodeToString(want))
	defer srv.Close()

	t.Setenv(vaultAddrEnv, srv.URL)
	t.Setenv(vaultTokenEnv, "test-token")
	t.Setenv(vaultKVPathEnv, "")

	got, err := ResolveMasterKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("key mismatch: got %x want %x", got, want)
	}
}

func TestResolveMasterKeyVaultCustomPath(t *testing.T) {
	want := testKey(0x33)
	const customPath = "secret/data/custom"
	srv := fakeVault(t, "/v1/"+customPath, base64.StdEncoding.EncodeToString(want))
	defer srv.Close()

	t.Setenv(vaultAddrEnv, srv.URL)
	t.Setenv(vaultTokenEnv, "test-token")
	t.Setenv(vaultKVPathEnv, customPath)

	got, err := ResolveMasterKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("key mismatch: got %x want %x", got, want)
	}
}

func TestResolveMasterKeyVaultFailsClosed(t *testing.T) {
	// Vault returns 500: must surface the error, never fall back to env/dev.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sealed", http.StatusInternalServerError)
	}))
	defer srv.Close()

	t.Setenv(vaultAddrEnv, srv.URL)
	t.Setenv(vaultTokenEnv, "test-token")
	t.Setenv(vaultKVPathEnv, "")
	// A valid env key is present; fail-closed means we must NOT use it.
	t.Setenv("REPO_CRED_KEY", base64.StdEncoding.EncodeToString(testKey(0x11)))

	if got, err := ResolveMasterKey(context.Background()); err == nil {
		t.Fatalf("expected fail-closed error, got key %x", got)
	}
}

func TestResolveMasterKeyVaultBadBase64(t *testing.T) {
	srv := fakeVault(t, "", "not!!base64!!")
	defer srv.Close()

	t.Setenv(vaultAddrEnv, srv.URL)
	t.Setenv(vaultTokenEnv, "test-token")
	t.Setenv(vaultKVPathEnv, "")

	if _, err := ResolveMasterKey(context.Background()); err == nil {
		t.Fatal("expected error for invalid base64 from Vault")
	}
}

func TestResolveMasterKeyVaultWrongLength(t *testing.T) {
	srv := fakeVault(t, "", base64.StdEncoding.EncodeToString([]byte("too-short")))
	defer srv.Close()

	t.Setenv(vaultAddrEnv, srv.URL)
	t.Setenv(vaultTokenEnv, "test-token")
	t.Setenv(vaultKVPathEnv, "")

	if _, err := ResolveMasterKey(context.Background()); err == nil {
		t.Fatal("expected error for non-32-byte key from Vault")
	}
}

func TestResolveMasterKeyVaultMissingField(t *testing.T) {
	srv := fakeVault(t, "", "") // empty repo_cred_key value
	defer srv.Close()

	t.Setenv(vaultAddrEnv, srv.URL)
	t.Setenv(vaultTokenEnv, "test-token")
	t.Setenv(vaultKVPathEnv, "")

	if _, err := ResolveMasterKey(context.Background()); err == nil {
		t.Fatal("expected error when repo_cred_key absent from Vault")
	}
}

func TestResolveMasterKeyDelegatesWhenVaultUnconfigured(t *testing.T) {
	// VAULT_ADDR/VAULT_TOKEN unset => delegate to crypto.MasterKey.
	t.Setenv(vaultAddrEnv, "")
	t.Setenv(vaultTokenEnv, "")

	// REPO_CRED_KEY set: expect that exact key.
	want := testKey(0x7e)
	t.Setenv("REPO_CRED_KEY", base64.StdEncoding.EncodeToString(want))
	got, err := ResolveMasterKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("key mismatch: got %x want %x", got, want)
	}

	// REPO_CRED_KEY unset: dev fallback (still 32 bytes, no error).
	t.Setenv("REPO_CRED_KEY", "")
	got, err = ResolveMasterKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on dev fallback: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("dev fallback key length = %d, want 32", len(got))
	}
}

func TestResolveMasterKeyVaultRequiresBothAddrAndToken(t *testing.T) {
	// Only VAULT_ADDR set (no token) => Vault NOT engaged, delegate to env.
	t.Setenv(vaultAddrEnv, "http://127.0.0.1:1")
	t.Setenv(vaultTokenEnv, "")
	want := testKey(0x21)
	t.Setenv("REPO_CRED_KEY", base64.StdEncoding.EncodeToString(want))

	got, err := ResolveMasterKey(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("key mismatch: got %x want %x", got, want)
	}
}
