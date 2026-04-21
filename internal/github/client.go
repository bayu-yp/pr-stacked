package github

import (
	"context"
	"fmt"
	"time"

	gogithub "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// Client wraps the go-github client with the methods needed by the engine.
type Client struct {
	gh    *gogithub.Client
	owner string
	repo  string
}

// New creates a new GitHub Client authenticated with the provided token.
// owner and repo are the default repository used when not explicitly specified.
func New(token, owner, repo string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	gh := gogithub.NewClient(tc)

	return &Client{
		gh:    gh,
		owner: owner,
		repo:  repo,
	}
}

// GetPR returns a single pull request.
func (c *Client) GetPR(ctx context.Context, owner, repo string, prNumber int) (*gogithub.PullRequest, error) {
	pr, resp, err := c.gh.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		if resp != nil && resp.StatusCode == 401 {
			return nil, fmt.Errorf("GetPR %d: GitHub returned 401 — check that GITHUB_TOKEN is valid and has 'repo' scope: %w", prNumber, err)
		}
		return nil, fmt.Errorf("GetPR %d: %w", prNumber, err)
	}

	return pr, nil
}

// UpdateBranch calls the GitHub "update-branch" API to merge the base branch
// HEAD into the PR's head branch.
func (c *Client) UpdateBranch(ctx context.Context, owner, repo string, prNumber int) error {
	opts := &gogithub.PullRequestBranchUpdateOptions{}
	_, _, err := c.gh.PullRequests.UpdateBranch(ctx, owner, repo, prNumber, opts)
	if err != nil {
		return fmt.Errorf("UpdateBranch PR#%d: %w", prNumber, err)
	}

	return nil
}

// PollMergeableState polls the PR's mergeable_state until it transitions away
// from "unknown". It polls up to maxPolls times with interval between each.
// Returns the final mergeable_state string.
func (c *Client) PollMergeableState(ctx context.Context, owner, repo string, prNumber, maxPolls int, interval time.Duration) (string, error) {
	for i := 0; i < maxPolls; i++ {
		pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, prNumber)
		if err != nil {
			return "", fmt.Errorf("PollMergeableState get PR#%d attempt %d: %w", prNumber, i+1, err)
		}

		state := pr.GetMergeableState()
		if state != "unknown" {
			return state, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}

	return "unknown", fmt.Errorf("mergeable_state for PR#%d still unknown after %d polls", prNumber, maxPolls)
}

// RetargetBase updates a PR's base branch to the given newBase.
func (c *Client) RetargetBase(ctx context.Context, owner, repo string, prNumber int, newBase string) error {
	update := &gogithub.PullRequestBranchUpdateOptions{}
	_ = update

	pr, _, err := c.gh.PullRequests.Edit(ctx, owner, repo, prNumber, &gogithub.PullRequest{
		Base: &gogithub.PullRequestBranch{
			Ref: gogithub.String(newBase),
		},
	})
	if err != nil {
		return fmt.Errorf("RetargetBase PR#%d to %q: %w", prNumber, newBase, err)
	}
	_ = pr

	return nil
}

// PostComment posts a comment on a pull request.
func (c *Client) PostComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	comment := &gogithub.IssueComment{
		Body: gogithub.String(body),
	}

	_, _, err := c.gh.Issues.CreateComment(ctx, owner, repo, prNumber, comment)
	if err != nil {
		return fmt.Errorf("PostComment PR#%d: %w", prNumber, err)
	}

	return nil
}

// ConflictComment returns the standard conflict notification comment body.
func ConflictComment(branchName, parentBranch string, parentPRNumber int, downstreamPRNumbers []int) string {
	downstream := ""
	if len(downstreamPRNumbers) > 0 {
		downstream = fmt.Sprintf("\n\nDownstream PRs (")
		for i, n := range downstreamPRNumbers {
			if i > 0 {
				downstream += ", "
			}
			downstream += fmt.Sprintf("#%d", n)
		}
		downstream += ") are paused until this is resolved."
	}

	return fmt.Sprintf(`[StackPR] Sync conflict detected

The branch `+"`%s`"+` could not be automatically updated from
`+"`%s`"+` (PR #%d) due to a merge conflict.

Please resolve locally:
  git checkout %s
  git merge %s
  # resolve conflicts
  git push
%s`, branchName, parentBranch, parentPRNumber, branchName, parentBranch, downstream)
}
