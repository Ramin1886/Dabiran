package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// In production, fetch this natively from a KMS or environment vault
var secretKey = []byte("super-secret-system-key-rotate-in-production")

// Claims extends standard JWT claims securely providing Team boundaries and RBAC roles tracking permissions internally.
type Claims struct {
	UserID int    `json:"user_id"`
	TeamID int    `json:"team_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed token matching the RBAC user payload securely enforcing 24h rotation bounds.
func GenerateToken(userID int, teamID int, role string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: userID,
		TeamID: teamID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// ValidateToken specifically decodes the cryptocraphic signature resolving the embedded claims structure isolating tenant arrays.
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid signature mapped natively")
	}

	return claims, nil
}
