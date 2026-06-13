// Package secrets resolves the AES-256 repository-credential master key from a
// secrets backend, layering HashiCorp Vault in front of the local
// crypto.MasterKey() sourcing (REPO_CRED_KEY env, then dev fallback).
package secrets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ramin1886/git-interactive-history/backend/src/crypto"
)

// Vault configuration environment variables. Vault is engaged only when both
// vaultAddrEnv and vaultTokenEnv are non-empty; otherwise resolution delegates
// to crypto.MasterKey().
const (
	vaultAddrEnv  = "VAULT_ADDR"
	vaultTokenEnv = "VAULT_TOKEN"
	// vaultKVPathEnv overrides the read path; the default targets a KV v2
	// secret at logical path "git-viz" in the "secret/" mount, whose data API
	// path is "secret/data/git-viz".
	vaultKVPathEnv  = "VAULT_KV_PATH"
	defaultKVPath   = "secret/data/git-viz"
	repoCredKeyName = "repo_cred_key"
)

// vaultTimeout bounds the single HTTP read against Vault so a hung backend
// cannot stall a credential operation indefinitely.
const vaultTimeout = 5 * time.Second

// kvV2Response models the subset of a Vault KV v2 read we consume:
// {"data":{"data":{"repo_cred_key":"<base64>"}}}.
type kvV2Response struct {
	Data struct {
		Data map[string]string `json:"data"`
	} `json:"data"`
}

// ResolveMasterKey returns the 32-byte AES-256 master key for repository
// credentials, resolved in the following priority order:
//
//  1. Vault — engaged when both VAULT_ADDR and VAULT_TOKEN are set. It performs
//     a GET against {VAULT_ADDR}/v1/{VAULT_KV_PATH} (VAULT_KV_PATH defaults to
//     the KV v2 data path "secret/data/git-viz") with an X-Vault-Token header,
//     reads .data.data.repo_cred_key (base64 of 32 bytes), and decodes it.
//     When Vault is configured it FAILS CLOSED: any Vault, decode, or length
//     error is returned as-is and resolution does NOT fall through to a weaker
//     source. A misconfigured Vault must never silently downgrade security by
//     reverting to the env/dev key.
//  2. Otherwise — when Vault is not configured — it delegates to
//     crypto.MasterKey() (REPO_CRED_KEY env, then the dev fallback with a
//     one-time warning).
//
// The returned key is always validated to be exactly 32 bytes.
func ResolveMasterKey(ctx context.Context) ([]byte, error) {
	addr := os.Getenv(vaultAddrEnv)
	token := os.Getenv(vaultTokenEnv)
	if addr != "" && token != "" {
		// Vault is explicitly configured: fail closed on any error.
		return resolveFromVault(ctx, addr, token, os.Getenv(vaultKVPathEnv))
	}
	// Vault not configured: delegate to the existing env/dev-key path.
	return crypto.MasterKey()
}

// resolveFromVault reads and decodes the master key from Vault. kvPath may be
// empty, in which case defaultKVPath is used. Every failure is returned (never
// swallowed) so the caller fails closed.
func resolveFromVault(ctx context.Context, addr, token, kvPath string) ([]byte, error) {
	if kvPath == "" {
		kvPath = defaultKVPath
	}

	ctx, cancel := context.WithTimeout(ctx, vaultTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/v1/%s", addr, kvPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("vault: build request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault: request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault: read %s returned HTTP %d", kvPath, resp.StatusCode)
	}

	var parsed kvV2Response
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("vault: decode response: %w", err)
	}

	enc := parsed.Data.Data[repoCredKeyName]
	if enc == "" {
		return nil, fmt.Errorf("vault: %q not found at %s", repoCredKeyName, kvPath)
	}

	key, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, errors.New("vault: repo_cred_key is not valid base64")
	}
	if len(key) != 32 {
		return nil, errors.New("vault: repo_cred_key must decode to exactly 32 bytes")
	}
	return key, nil
}
