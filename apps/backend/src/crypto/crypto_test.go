package crypto

import (
	"bytes"
	"encoding/base64"
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

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey(0x42)
	plaintext := []byte("ghp_super_secret_personal_access_token")

	enc, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if enc == string(plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(dec, plaintext) {
		t.Fatalf("round trip mismatch: got %q want %q", dec, plaintext)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	enc, err := Encrypt([]byte("payload"), testKey(0x01))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if _, err := Decrypt(enc, testKey(0x02)); err == nil {
		t.Fatal("Decrypt with wrong key should fail")
	}
}

func TestDecryptShortCiphertextFails(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02})
	if _, err := Decrypt(short, testKey(0x01)); err == nil {
		t.Fatal("Decrypt of too-short ciphertext should fail")
	}
}

func TestKeyLengthValidation(t *testing.T) {
	if _, err := Encrypt([]byte("x"), []byte("short-key")); err == nil {
		t.Fatal("Encrypt with non-32-byte key should fail")
	}
	if _, err := Decrypt("ignored", []byte("short-key")); err == nil {
		t.Fatal("Decrypt with non-32-byte key should fail")
	}
}
