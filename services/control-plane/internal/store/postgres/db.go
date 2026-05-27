package postgres

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_initial.sql
var migrationSQL1 string

//go:embed migrations/002_phase2.sql
var migrationSQL2 string

//go:embed migrations/003_phase3.sql
var migrationSQL3 string

//go:embed migrations/004_phase4.sql
var migrationSQL4 string

//go:embed migrations/005_phase5.sql
var migrationSQL5 string

//go:embed migrations/006_embed.sql
var migrationSQL6 string

// Store wraps a pgxpool.Pool and provides all database operations.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to Postgres using the provided DSN, runs the embedded migration
// SQL to ensure the schema is up to date, and returns a ready Store.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	for i, sql := range []string{migrationSQL1, migrationSQL2, migrationSQL3, migrationSQL4, migrationSQL5, migrationSQL6} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			pool.Close()
			return nil, fmt.Errorf("running migration %d: %w", i+1, err)
		}
	}

	return &Store{pool: pool}, nil
}

// Close releases all pool connections.
func (s *Store) Close() {
	s.pool.Close()
}
