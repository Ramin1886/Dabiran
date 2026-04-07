package models

import (
	"time"
)

// User represents a platform tenant member.
type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"` // admin, member
	CreatedAt time.Time `json:"created_at"`
}

// Team represents an organizational unit.
type Team struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	OwnerID   int       `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository stores metadata about a connected Git repository.
// Credentials (PATs/SSH) are stored encrypted.
type Repository struct {
	ID                  int       `json:"id"`
	TeamID              int       `json:"team_id"`
	Name                string    `json:"name"`
	URL                 string    `json:"url"`
	EncryptedCredential string    `json:"-"` // Never serialize
	CreatedAt           time.Time `json:"created_at"`
}

// Annotation represents a custom visual link or text placed on the canvas.
type Annotation struct {
	ID           int       `json:"id"`
	RepositoryID int       `json:"repository_id"`
	Type         string    `json:"type"` // e.g., 'text', 'line'
	Payload      string    `json:"payload"` // JSON structure of x/y coords, content
	AuthorID     int       `json:"author_id"`
	CreatedAt    time.Time `json:"created_at"`
}
