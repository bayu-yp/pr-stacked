package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Stack represents a row in the stacks table.
type Stack struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RepoOwner string    `json:"repo_owner"`
	RepoName  string    `json:"repo_name"`
	CreatedAt time.Time `json:"created_at"`
}

// StackEntry represents a row in the stack_entries table.
type StackEntry struct {
	ID         string     `json:"id"`
	StackID    string     `json:"stack_id"`
	PRNumber   int        `json:"pr_number"`
	BranchName string     `json:"branch_name"`
	Position   int        `json:"position"`
	Status     string     `json:"status"`
	LastSynced *time.Time `json:"last_synced"`
}

// SyncEvent represents a row in the sync_events table.
type SyncEvent struct {
	ID           string     `json:"id"`
	StackID      string     `json:"stack_id"`
	TriggeredBy  int        `json:"triggered_by"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message"`
}

// CreateStack inserts a new stack and returns the created row.
func CreateStack(ctx context.Context, name, repoOwner, repoName string) (*Stack, error) {
	const q = `
		INSERT INTO stacks (name, repo_owner, repo_name)
		VALUES ($1, $2, $3)
		RETURNING id, name, repo_owner, repo_name, created_at
	`

	row := Pool.QueryRow(ctx, q, name, repoOwner, repoName)

	var s Stack
	if err := row.Scan(&s.ID, &s.Name, &s.RepoOwner, &s.RepoName, &s.CreatedAt); err != nil {
		return nil, fmt.Errorf("CreateStack: %w", err)
	}

	return &s, nil
}

// GetStackByName returns a stack matching name, owner, and repo.
func GetStackByName(ctx context.Context, name, repoOwner, repoName string) (*Stack, error) {
	const q = `
		SELECT id, name, repo_owner, repo_name, created_at
		FROM stacks
		WHERE name = $1 AND repo_owner = $2 AND repo_name = $3
	`

	row := Pool.QueryRow(ctx, q, name, repoOwner, repoName)

	var s Stack
	if err := row.Scan(&s.ID, &s.Name, &s.RepoOwner, &s.RepoName, &s.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("GetStackByName: %w", err)
	}

	return &s, nil
}

// GetStackByID returns a stack by its UUID.
func GetStackByID(ctx context.Context, id string) (*Stack, error) {
	const q = `
		SELECT id, name, repo_owner, repo_name, created_at
		FROM stacks
		WHERE id = $1
	`

	row := Pool.QueryRow(ctx, q, id)

	var s Stack
	if err := row.Scan(&s.ID, &s.Name, &s.RepoOwner, &s.RepoName, &s.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("GetStackByID: %w", err)
	}

	return &s, nil
}

// ListStacks returns all stacks across all repos, grouped by repo.
func ListStacks(ctx context.Context) ([]*Stack, error) {
	const q = `
		SELECT id, name, repo_owner, repo_name, created_at
		FROM stacks
		ORDER BY repo_owner, repo_name, created_at ASC
	`

	rows, err := Pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ListStacks: %w", err)
	}
	defer rows.Close()

	var stacks []*Stack
	for rows.Next() {
		var s Stack
		if err := rows.Scan(&s.ID, &s.Name, &s.RepoOwner, &s.RepoName, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("ListStacks scan: %w", err)
		}
		stacks = append(stacks, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListStacks rows: %w", err)
	}

	return stacks, nil
}

// GetMaxPosition returns the current maximum position within a stack, or -1 if the stack is empty.
func GetMaxPosition(ctx context.Context, stackID string) (int, error) {
	const q = `SELECT COALESCE(MAX(position), -1) FROM stack_entries WHERE stack_id = $1`

	var max int
	if err := Pool.QueryRow(ctx, q, stackID).Scan(&max); err != nil {
		return -1, fmt.Errorf("GetMaxPosition: %w", err)
	}

	return max, nil
}

// AddStackEntry inserts a new entry into a stack.
func AddStackEntry(ctx context.Context, stackID string, prNumber int, branchName string, position int) (*StackEntry, error) {
	const q = `
		INSERT INTO stack_entries (stack_id, pr_number, branch_name, position)
		VALUES ($1, $2, $3, $4)
		RETURNING id, stack_id, pr_number, branch_name, position, status, last_synced
	`

	row := Pool.QueryRow(ctx, q, stackID, prNumber, branchName, position)

	var e StackEntry
	if err := row.Scan(&e.ID, &e.StackID, &e.PRNumber, &e.BranchName, &e.Position, &e.Status, &e.LastSynced); err != nil {
		return nil, fmt.Errorf("AddStackEntry: %w", err)
	}

	return &e, nil
}

// RemoveStackEntry removes an entry by PR number and stack ID, then compacts positions.
func RemoveStackEntry(ctx context.Context, stackID string, prNumber int) error {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("RemoveStackEntry begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Get the position of the entry being removed.
	var removedPosition int
	err = tx.QueryRow(ctx,
		`SELECT position FROM stack_entries WHERE stack_id = $1 AND pr_number = $2`,
		stackID, prNumber,
	).Scan(&removedPosition)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("PR #%d is not in this stack", prNumber)
		}
		return fmt.Errorf("RemoveStackEntry query position: %w", err)
	}

	// Delete the target entry.
	if _, err := tx.Exec(ctx,
		`DELETE FROM stack_entries WHERE stack_id = $1 AND pr_number = $2`,
		stackID, prNumber,
	); err != nil {
		return fmt.Errorf("RemoveStackEntry delete: %w", err)
	}

	// Shift down all entries that had a higher position.
	if _, err := tx.Exec(ctx,
		`UPDATE stack_entries SET position = position - 1 WHERE stack_id = $1 AND position > $2`,
		stackID, removedPosition,
	); err != nil {
		return fmt.Errorf("RemoveStackEntry compact positions: %w", err)
	}

	return tx.Commit(ctx)
}

// GetEntriesFromPosition returns all entries at position >= fromPosition, ordered by position.
func GetEntriesFromPosition(ctx context.Context, stackID string, fromPosition int) ([]*StackEntry, error) {
	const q = `
		SELECT id, stack_id, pr_number, branch_name, position, status, last_synced
		FROM stack_entries
		WHERE stack_id = $1 AND position >= $2
		ORDER BY position ASC
	`

	rows, err := Pool.Query(ctx, q, stackID, fromPosition)
	if err != nil {
		return nil, fmt.Errorf("GetEntriesFromPosition: %w", err)
	}
	defer rows.Close()

	var entries []*StackEntry
	for rows.Next() {
		var e StackEntry
		if err := rows.Scan(&e.ID, &e.StackID, &e.PRNumber, &e.BranchName, &e.Position, &e.Status, &e.LastSynced); err != nil {
			return nil, fmt.Errorf("GetEntriesFromPosition scan: %w", err)
		}
		entries = append(entries, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetEntriesFromPosition rows: %w", err)
	}

	return entries, nil
}

// GetAllEntries returns all entries for a stack ordered by position.
func GetAllEntries(ctx context.Context, stackID string) ([]*StackEntry, error) {
	const q = `
		SELECT id, stack_id, pr_number, branch_name, position, status, last_synced
		FROM stack_entries
		WHERE stack_id = $1
		ORDER BY position ASC
	`

	rows, err := Pool.Query(ctx, q, stackID)
	if err != nil {
		return nil, fmt.Errorf("GetAllEntries: %w", err)
	}
	defer rows.Close()

	var entries []*StackEntry
	for rows.Next() {
		var e StackEntry
		if err := rows.Scan(&e.ID, &e.StackID, &e.PRNumber, &e.BranchName, &e.Position, &e.Status, &e.LastSynced); err != nil {
			return nil, fmt.Errorf("GetAllEntries scan: %w", err)
		}
		entries = append(entries, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetAllEntries rows: %w", err)
	}

	return entries, nil
}

// UpdateEntryStatus updates the status and last_synced timestamp for a stack entry.
func UpdateEntryStatus(ctx context.Context, entryID string, status string) error {
	const q = `
		UPDATE stack_entries
		SET status = $1, last_synced = NOW()
		WHERE id = $2
	`

	if _, err := Pool.Exec(ctx, q, status, entryID); err != nil {
		return fmt.Errorf("UpdateEntryStatus: %w", err)
	}

	return nil
}

// GetEntryByPRNumber finds a stack entry by PR number across all stacks for a given repo.
func GetEntryByPRNumber(ctx context.Context, prNumber int, repoOwner, repoName string) (*StackEntry, *Stack, error) {
	const q = `
		SELECT e.id, e.stack_id, e.pr_number, e.branch_name, e.position, e.status, e.last_synced,
		       s.id, s.name, s.repo_owner, s.repo_name, s.created_at
		FROM stack_entries e
		JOIN stacks s ON s.id = e.stack_id
		WHERE e.pr_number = $1 AND s.repo_owner = $2 AND s.repo_name = $3
		LIMIT 1
	`

	row := Pool.QueryRow(ctx, q, prNumber, repoOwner, repoName)

	var e StackEntry
	var s Stack
	err := row.Scan(
		&e.ID, &e.StackID, &e.PRNumber, &e.BranchName, &e.Position, &e.Status, &e.LastSynced,
		&s.ID, &s.Name, &s.RepoOwner, &s.RepoName, &s.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("GetEntryByPRNumber: %w", err)
	}

	return &e, &s, nil
}

// MarkEntryMerged sets an entry's status to 'merged'.
func MarkEntryMerged(ctx context.Context, entryID string) error {
	const q = `UPDATE stack_entries SET status = 'merged', last_synced = NOW() WHERE id = $1`

	if _, err := Pool.Exec(ctx, q, entryID); err != nil {
		return fmt.Errorf("MarkEntryMerged: %w", err)
	}

	return nil
}

// WriteSyncEvent inserts a sync_events record.
func WriteSyncEvent(ctx context.Context, stackID string, triggeredBy int, status, errMsg string) error {
	const q = `
		INSERT INTO sync_events (stack_id, triggered_by, started_at, finished_at, status, error_message)
		VALUES ($1, $2, NOW(), NOW(), $3, $4)
	`

	if _, err := Pool.Exec(ctx, q, stackID, triggeredBy, status, errMsg); err != nil {
		return fmt.Errorf("WriteSyncEvent: %w", err)
	}

	return nil
}

// ListSyncEvents returns up to limit sync events for a stack, newest first.
func ListSyncEvents(ctx context.Context, stackID string, limit int) ([]*SyncEvent, error) {
	const q = `
		SELECT id, stack_id, triggered_by, started_at, finished_at, status, error_message
		FROM sync_events
		WHERE stack_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`

	rows, err := Pool.Query(ctx, q, stackID, limit)
	if err != nil {
		return nil, fmt.Errorf("ListSyncEvents: %w", err)
	}
	defer rows.Close()

	var events []*SyncEvent
	for rows.Next() {
		var e SyncEvent
		if err := rows.Scan(&e.ID, &e.StackID, &e.TriggeredBy, &e.StartedAt, &e.FinishedAt, &e.Status, &e.ErrorMessage); err != nil {
			return nil, fmt.Errorf("ListSyncEvents scan: %w", err)
		}
		events = append(events, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListSyncEvents rows: %w", err)
	}

	return events, nil
}

// RetargetChildEntry updates stack entries so the child PR at removedPosition+1
// becomes the new root (position 0) by decrementing all positions >= removedPosition.
// This is called after a PR is merged and its entry removed from the stack.
func RetargetChildEntry(ctx context.Context, stackID string, mergedPosition int) error {
	const q = `
		UPDATE stack_entries
		SET position = position - 1
		WHERE stack_id = $1 AND position > $2
	`

	if _, err := Pool.Exec(ctx, q, stackID, mergedPosition); err != nil {
		return fmt.Errorf("RetargetChildEntry: %w", err)
	}

	return nil
}
