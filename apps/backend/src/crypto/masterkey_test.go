package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestMasterKey(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(testKey(0x7f))
	tests := []struct {
		name    string
		env     string // value to set REPO_CRED_KEY to; "-" means leave unset
		wantErr bool
		want    []byte // expected key when no error (nil = don't compare)
	}{
		{name: "unset falls back to dev key", env: "-", wantErr: false, want: devFallbackMasterKey},
		{name: "valid base64 32 bytes", env: valid, wantErr: false, want: testKey(0x7f)},
		{name: "invalid base64", env: "not!!base64!!", wantErr: true},
		{name: "wrong length", env: base64.StdEncoding.EncodeToString([]byte("too-short")), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env == "-" {
				t.Setenv(repoCredKeyEnv, "")
			} else {
				t.Setenv(repoCredKeyEnv, tc.env)
			}
			got, err := MasterKey()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got key %x", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 32 {
				t.Fatalf("key length = %d, want 32", len(got))
			}
			if tc.want != nil && !bytes.Equal(got, tc.want) {
				t.Fatalf("key mismatch: got %x want %x", got, tc.want)
			}
		})
	}
}

func TestMasterKeyRoundTripsWithEncrypt(t *testing.T) {
	t.Setenv(repoCredKeyEnv, "")
	key, err := MasterKey()
	if err != nil {
		t.Fatalf("MasterKey: %v", err)
	}
	enc, err := Encrypt([]byte("ghp_token"), key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(dec) != "ghp_token" {
		t.Fatalf("round trip mismatch: %q", dec)
	}
}
