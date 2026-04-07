package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

// Initialize sets up the global database connection pool
func Initialize(ctx context.Context, dbURL string) error {
	var err error
	Pool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	return migrate(ctx)
}

func migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS teams (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			owner_id INT REFERENCES users(id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS repositories (
			id SERIAL PRIMARY KEY,
			team_id INT REFERENCES teams(id),
			name VARCHAR(255) NOT NULL,
			url VARCHAR(512) NOT NULL,
			encrypted_credential TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS annotations (
			id SERIAL PRIMARY KEY,
			repository_id INT REFERENCES repositories(id),
			type VARCHAR(50) NOT NULL,
			payload JSONB NOT NULL,
			author_id INT REFERENCES users(id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, query := range queries {
		_, err := Pool.Exec(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	return nil
}

// Close terminates the pool connection
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}
