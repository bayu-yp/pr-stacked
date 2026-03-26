package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the shared connection pool used by all repository functions.
var Pool *pgxpool.Pool

// Connect initialises the connection pool from DATABASE_URL env var.
// Call this once during application startup.
func Connect(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	Pool = pool
	return nil
}

// Migrate applies the DDL for all tables if they do not already exist.
// It is idempotent and safe to call on every startup.
func Migrate(ctx context.Context) error {
	if Pool == nil {
		return fmt.Errorf("database pool is not initialised; call Connect first")
	}

	ddl := `
CREATE TABLE IF NOT EXISTS stacks (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  repo_owner  TEXT NOT NULL,
  repo_name   TEXT NOT NULL,
  created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stack_entries (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stack_id    UUID NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
  pr_number   INTEGER NOT NULL,
  branch_name TEXT NOT NULL,
  position    INTEGER NOT NULL,
  status      TEXT NOT NULL DEFAULT 'synced'
                CHECK (status IN ('synced', 'conflict', 'pending', 'merged')),
  last_synced TIMESTAMPTZ,
  UNIQUE(stack_id, position)
);

CREATE TABLE IF NOT EXISTS sync_events (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  stack_id      UUID NOT NULL REFERENCES stacks(id),
  triggered_by  INTEGER NOT NULL,
  started_at    TIMESTAMPTZ NOT NULL,
  finished_at   TIMESTAMPTZ,
  status        TEXT NOT NULL CHECK (status IN ('success', 'partial', 'failed')),
  error_message TEXT
);
`

	if _, err := Pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// Close gracefully closes the connection pool.
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}
