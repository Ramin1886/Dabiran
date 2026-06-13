package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndValidateToken(t *testing.T) {
	token, err := GenerateToken(7, DefaultTeamID, "Team Member")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != 7 || claims.TeamID != DefaultTeamID || claims.Role != "Team Member" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	now := time.Now()
	claims := &Claims{
		UserID: 1, TeamID: DefaultTeamID, Role: "Team Owner",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-25 * time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secretKey())
	if err != nil {
		t.Fatalf("signing expired token failed: %v", err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Fatal("expired token should be rejected")
	}
}

func TestValidateWrongSignature(t *testing.T) {
	claims := &Claims{
		UserID: 1, TeamID: DefaultTeamID, Role: "Team Owner",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("attacker-controlled-key"))
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Fatal("token with wrong signature should be rejected")
	}
}

func TestSecretFromEnvironment(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-secret")
	token, err := GenerateToken(1, DefaultTeamID, "Admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if _, err := ValidateToken(token); err != nil {
		t.Fatalf("token signed with env secret should validate: %v", err)
	}

	// A token signed under one secret must fail once the secret rotates.
	t.Setenv("JWT_SECRET", "rotated-secret")
	if _, err := ValidateToken(token); err == nil {
		t.Fatal("token should be invalid after secret rotation")
	}
}
