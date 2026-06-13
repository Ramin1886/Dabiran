// Package crypto provides AES-256-GCM symmetric encryption for tenant
// credentials (PATs/SSH keys) before they are persisted.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypt seals plaintext with AES-256-GCM under the 32-byte key and returns
// base64(nonce || ciphertext). A fresh random nonce is generated per call.
func Encrypt(plaintext []byte, key []byte) (string, error) {
	if len(key) != 32 {
		return "", errors.New("AES-256 requires a 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt: it decodes base64(nonce || ciphertext) and opens
// it with the 32-byte key, returning the plaintext or an error for malformed
// input, truncated ciphertext, or authentication (wrong key) failure.
func Decrypt(encString string, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256 requires a 32-byte key")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encString)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return aesGCM.Open(nil, nonce, ciphertext, nil)
}
