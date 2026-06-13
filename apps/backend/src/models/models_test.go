package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRepositoryNeverSerializesCredential(t *testing.T) {
	repo := Repository{
		ID:                  1,
		TeamID:              100,
		Name:                "core",
		URL:                 "https://github.com/example/core.git",
		EncryptedCredential: "TOP-SECRET-CIPHERTEXT",
		CreatedAt:           time.Now(),
	}
	data, err := json.Marshal(repo)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "TOP-SECRET-CIPHERTEXT") {
		t.Fatalf("encrypted credential leaked into JSON: %s", data)
	}
}

func TestModelsMarshalRoundTrip(t *testing.T) {
	user := User{ID: 1, Email: "ram@example.com", Name: "Ram", Role: "Admin", CreatedAt: time.Now().UTC()}
	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}
	var back User
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	if back.Email != user.Email || back.Role != user.Role {
		t.Fatalf("round trip mismatch: %+v vs %+v", back, user)
	}
}
