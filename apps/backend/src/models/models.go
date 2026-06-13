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
type Repository struct {
	ID                  int
	TeamID              int
	Name                string
	URL                 string
	EncryptedCredential string `json:"-"`
	CreatedAt           time.Time
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
