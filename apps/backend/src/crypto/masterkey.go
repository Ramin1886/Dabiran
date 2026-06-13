package crypto

import (
	"encoding/base64"
	"errors"
	"log"
	"os"
	"sync"
)

// repoCredKeyEnv names the environment variable holding the base64-encoded
// 32-byte master key used to seal repository credentials at rest.
const repoCredKeyEnv = "REPO_CRED_KEY"

// devFallbackMasterKey is a fixed 32-byte key used only when REPO_CRED_KEY is
// unset, so local development without a configured secret store still works.
// PRODUCTION MUST set REPO_CRED_KEY (this is the seam where a Vault-sourced
// key will later plug in); credentials sealed under this dev key are NOT safe.
var devFallbackMasterKey = []byte("dev-master-key-32-bytes-rotate!!")

// warnDevFallbackOnce ensures the insecure-fallback warning is logged at most
// once per process to avoid log spam on every credential operation.
var warnDevFallbackOnce sync.Once

// MasterKey returns the 32-byte master key used with Encrypt/Decrypt for
// repository credentials. It decodes the base64 value of the REPO_CRED_KEY
// environment variable; when that variable is unset it logs a one-time
// warning and returns the documented dev fallback. It errors when the
// variable is set but malformed (invalid base64 or wrong length).
func MasterKey() ([]byte, error) {
	enc := os.Getenv(repoCredKeyEnv)
	if enc == "" {
		warnDevFallbackOnce.Do(func() {
			log.Printf("WARNING: %s unset; using insecure dev master key. Set %s (base64 32 bytes) in production.", repoCredKeyEnv, repoCredKeyEnv)
		})
		return devFallbackMasterKey, nil
	}
	key, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, errors.New("REPO_CRED_KEY is not valid base64")
	}
	if len(key) != 32 {
		return nil, errors.New("REPO_CRED_KEY must decode to exactly 32 bytes")
	}
	return key, nil
}
