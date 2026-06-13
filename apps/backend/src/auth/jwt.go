package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// devFallbackSecret is the documented local-development signing key, used
// only when JWT_SECRET is unset (see apps/backend/.env.example).
const devFallbackSecret = "super-secret-system-key-rotate-in-production"

// Single-tenant defaults. The deployment currently provisions exactly one
// implicit team (and one mock user); every issued token and every tenancy
// check derives from these constants until real user/team persistence lands.
const (
	DefaultUserID = 1
	DefaultTeamID = 100
)

// secretKey returns the HS256 signing key from the JWT_SECRET environment
// variable, falling back to the dev secret. Resolved per call so tests and
// runtime reconfiguration take effect without process restarts.
func secretKey() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte(devFallbackSecret)
}

// Claims extends the standard JWT registered claims with the tenant (team)
// boundary and RBAC role used by the API middleware.
type Claims struct {
	UserID int    `json:"user_id"`
	TeamID int    `json:"team_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken signs a 24h HS256 token embedding the user, team, and role
// claims. Returns the compact JWT string or a signing error.
func GenerateToken(userID int, teamID int, role string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		TeamID: teamID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secretKey())
}

// ValidateToken parses and verifies tokenString (signature and expiry) and
// returns its Claims, or an error for any invalid, expired, or tampered token.
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return secretKey(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
