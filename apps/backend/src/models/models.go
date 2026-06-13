// Package models defines the persisted entities; the relational schema in
// src/db mirrors these structs one-to-one.
package models

import "time"

// User is an authenticated platform account with a global RBAC role.
type User struct {
	ID        int
	Email     string
	Name      string
	Role      string
	CreatedAt time.Time
}

// Team is the tenancy boundary; repositories bind to exactly one team (1:N).
type Team struct {
	ID        int
	Name      string
	OwnerID   int
	CreatedAt time.Time
}

// Repository is a tracked remote git repository owned by a team. The
// credential is stored AES-256-GCM encrypted and never serialized to JSON.
// AuthType selects the transport auth scheme ("https", "ssh", or "").
type Repository struct {
	ID                  int
	TeamID              int
	Name                string
	URL                 string
	AuthType            string
	EncryptedCredential string `json:"-"`
	CreatedAt           time.Time
}

// TeamMembership binds a user to a team with a per-team RBAC role. The
// (team_id, user_id) pair is unique, so a user holds at most one role per team.
type TeamMembership struct {
	ID     int
	TeamID int
	UserID int
	Role   string
}

// Annotation is a persisted canvas annotation (snapshot of CRDT state)
// attached to a repository by an author.
type Annotation struct {
	ID           int
	RepositoryID int
	Type         string
	Payload      string
	AuthorID     int
	CreatedAt    time.Time
}

// CanvasView is a user's named snapshot of the frontend canvas view state.
// State is an opaque, frontend-owned JSON string (serialized viewport + active
// filters) the backend persists verbatim and never parses. Views are scoped to
// the owning user (UserID) within a team (TeamID).
type CanvasView struct {
	ID        int
	UserID    int
	TeamID    int
	Name      string
	State     string
	CreatedAt time.Time
}
