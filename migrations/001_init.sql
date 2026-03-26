-- Migration: 001_init
-- Creates the initial StackPR schema.

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
