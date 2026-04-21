package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/stackpr/stackpr/internal/db"
	"github.com/stackpr/stackpr/internal/github"
)

const (
	maxMergeablePolls    = 10
	mergeablePollInterval = 3 * time.Second
)

// Engine orchestrates cascade sync, conflict detection, and base retargeting.
type Engine struct {
	gh *github.Client
}

// New creates an Engine with the provided GitHub client.
func New(gh *github.Client) *Engine {
	return &Engine{gh: gh}
}

// CascadeSync propagates a merge update from the entry at fromPosition downward
// to all child entries in the stack. It stops if a conflict is detected at any node.
//
// fromPosition should be the position of the PR that was just updated/merged.
// The cascade applies to entries at positions >= fromPosition+1.
func (e *Engine) CascadeSync(ctx context.Context, stack *db.Stack, fromPosition, triggeredByPR int) error {
	// Load all entries that need to be synced (children of the updated entry).
	entries, err := db.GetEntriesFromPosition(ctx, stack.ID, fromPosition+1)
	if err != nil {
		return fmt.Errorf("CascadeSync load entries: %w", err)
	}

	if len(entries) == 0 {
		// Nothing to sync.
		return db.WriteSyncEvent(ctx, stack.ID, triggeredByPR, "success", "")
	}

	// We need the parent entry to build conflict comments.
	parentEntries, err := db.GetEntriesFromPosition(ctx, stack.ID, fromPosition)
	if err != nil {
		return fmt.Errorf("CascadeSync load parent entry: %w", err)
	}

	var parentEntry *db.StackEntry
	if len(parentEntries) > 0 && parentEntries[0].Position == fromPosition {
		parentEntry = parentEntries[0]
	}

	for i, entry := range entries {
		if entry.Status == "merged" {
			// Safety net: skip entries that were marked merged but not yet removed.
			continue
		}

		// Fetch the live PR state so we can detect merges that happened outside
		// the webhook flow (e.g. manual merges while the server was down).
		ghPR, err := e.gh.GetPR(ctx, stack.RepoOwner, stack.RepoName, entry.PRNumber)
		if err != nil {
			errMsg := fmt.Sprintf("failed to fetch PR#%d: %v", entry.PRNumber, err)
			_ = db.UpdateEntryStatus(ctx, entry.ID, "pending")
			_ = db.WriteSyncEvent(ctx, stack.ID, triggeredByPR, "partial", errMsg)
			return fmt.Errorf("%s", errMsg)
		}

		if ghPR.GetMerged() {
			// PR was merged outside the normal flow. Mark it and hand off to
			// RetargetBase, which will retarget the next open PR's base branch
			// and re-run a cascade sync for the remainder of the stack.
			_ = db.MarkEntryMerged(ctx, entry.ID)
			return e.RetargetBase(ctx, stack, entry, ghPR.GetBase().GetRef())
		}

		if err := e.gh.UpdateBranch(ctx, stack.RepoOwner, stack.RepoName, entry.PRNumber); err != nil {
			// UpdateBranch can return a 422 when the branch is already up-to-date.
			// Treat that as clean rather than an error.
			fmt.Printf("[engine] UpdateBranch PR#%d: %v (treating as up-to-date)\n", entry.PRNumber, err)
		}

		state, err := e.gh.PollMergeableState(
			ctx,
			stack.RepoOwner, stack.RepoName,
			entry.PRNumber,
			maxMergeablePolls, mergeablePollInterval,
		)
		if err != nil {
			errMsg := fmt.Sprintf("failed to determine mergeable_state for PR#%d: %v", entry.PRNumber, err)
			_ = db.UpdateEntryStatus(ctx, entry.ID, "pending")
			_ = db.WriteSyncEvent(ctx, stack.ID, triggeredByPR, "partial", errMsg)
			return fmt.Errorf("%s", errMsg)
		}

		switch state {
		case "dirty":
			_ = db.UpdateEntryStatus(ctx, entry.ID, "conflict")

			// Build downstream PR list for the comment.
			var downstream []int
			for _, d := range entries[i+1:] {
				downstream = append(downstream, d.PRNumber)
			}

			parentBranch := ""
			parentPRNumber := 0
			if parentEntry != nil {
				parentBranch = parentEntry.BranchName
				parentPRNumber = parentEntry.PRNumber
			}

			comment := github.ConflictComment(entry.BranchName, parentBranch, parentPRNumber, downstream)
			_ = e.gh.PostComment(ctx, stack.RepoOwner, stack.RepoName, entry.PRNumber, comment)

			errMsg := fmt.Sprintf("conflict detected at PR#%d (position %d)", entry.PRNumber, entry.Position)
			_ = db.WriteSyncEvent(ctx, stack.ID, triggeredByPR, "partial", errMsg)
			return nil // cascade halts; this is not a processing error

		case "clean":
			if err := db.UpdateEntryStatus(ctx, entry.ID, "synced"); err != nil {
				return fmt.Errorf("CascadeSync update status: %w", err)
			}

		default:
			// Unexpected state (e.g. "blocked", "behind") — mark pending and continue.
			_ = db.UpdateEntryStatus(ctx, entry.ID, "pending")
		}

		// Advance the parent pointer so the next iteration has the right parent context.
		parentEntry = entry
	}

	return db.WriteSyncEvent(ctx, stack.ID, triggeredByPR, "success", "")
}

// RetargetBase is called when the PR at mergedPosition is merged.
// It retargets the immediate child PR's base branch to the stack's root target
// (e.g. dev/main), then triggers a cascade sync.
//
// rootBase is the branch that the merged PR was targeting (e.g. "dev").
func (e *Engine) RetargetBase(ctx context.Context, stack *db.Stack, mergedEntry *db.StackEntry, rootBase string) error {
	// Find the immediate child entry.
	childEntries, err := db.GetEntriesFromPosition(ctx, stack.ID, mergedEntry.Position+1)
	if err != nil {
		return fmt.Errorf("RetargetBase load child entries: %w", err)
	}

	// Always remove the merged entry from the stack and compact positions,
	// even if it was the last entry (no children). Leaving it in the DB would
	// cause CascadeSync to try UpdateBranch on an already-merged PR.
	if err := db.RemoveStackEntry(ctx, stack.ID, mergedEntry.PRNumber); err != nil {
		return fmt.Errorf("RetargetBase remove merged entry: %w", err)
	}

	if len(childEntries) == 0 {
		// Merged entry was the last in the stack; nothing to retarget.
		return nil
	}

	child := childEntries[0]

	// Check if the child is already merged on GitHub.
	// This enables webhook-free chained-merge handling: running
	// `stackpr stack merged <pr>` (or the server receiving one merged event)
	// will automatically walk the entire chain of already-merged PRs and
	// retarget / clean up each one without needing separate webhook deliveries.
	childPR, err := e.gh.GetPR(ctx, stack.RepoOwner, stack.RepoName, child.PRNumber)
	if err != nil {
		return fmt.Errorf("RetargetBase fetch child PR#%d: %w", child.PRNumber, err)
	}

	if childPR.GetMerged() {
		// Child was already merged on GitHub; record it and recurse down the chain.
		if err := db.MarkEntryMerged(ctx, child.ID); err != nil {
			return fmt.Errorf("RetargetBase mark child PR#%d merged: %w", child.PRNumber, err)
		}
		// Re-fetch the child entry to get its compacted position (the parent was
		// just removed and positions shifted down by one).
		updatedChild, _, err := db.GetEntryByPRNumber(ctx, child.PRNumber, stack.RepoOwner, stack.RepoName)
		if err != nil {
			return fmt.Errorf("RetargetBase re-fetch child PR#%d: %w", child.PRNumber, err)
		}
		if updatedChild == nil {
			return nil
		}
		return e.RetargetBase(ctx, stack, updatedChild, rootBase)
	}

	// Child is still open: retarget its base to the root base branch.
	if err := e.gh.RetargetBase(ctx, stack.RepoOwner, stack.RepoName, child.PRNumber, rootBase); err != nil {
		return fmt.Errorf("RetargetBase update PR#%d: %w", child.PRNumber, err)
	}

	// After compaction, the child is now at position 0 (the new root).
	// Cascade sync from position 0.
	return e.CascadeSync(ctx, stack, 0, mergedEntry.PRNumber)
}

// MarkStackEntryBroken sets the entry status to 'conflict' to indicate a PR was
// closed without merging, which breaks the downstream chain.
func (e *Engine) MarkStackEntryBroken(ctx context.Context, entry *db.StackEntry) error {
	return db.UpdateEntryStatus(ctx, entry.ID, "conflict")
}
