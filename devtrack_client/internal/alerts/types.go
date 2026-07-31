package alerts

import "time"

// ReviewCommentEvent is emitted by the alert poller when a reviewer leaves a
// new comment on a PR authored by the developer. The infra layer stores this
// in pr_review_comments and then calls /review/classify.
type ReviewCommentEvent struct {
	Platform    string // "github" | "azure" | "gitlab"
	Workspace   string
	PRID        string // PR number or platform PR ID
	PRTitle     string
	CommentID   string // platform-native comment ID (for idempotency)
	CommentBody string
	Reviewer    string
	CommentURL  string
	DetectedAt  time.Time
}

// MergedPREvent is emitted by the alert poller when a PR authored by the
// developer is merged into the repo's default branch (TASK-126). The daemon
// converts this into a commit trigger with is_merge_to_default=true so the
// Python server stages a done state-transition for the ticket.
type MergedPREvent struct {
	Platform   string // "github" (azure/gitlab: not yet implemented)
	Workspace  string
	PRID       string
	PRTitle    string
	HeadBranch string // the merged branch — carries the ticket ID
	BaseBranch string // the default branch the PR merged into
	MergeSHA   string
	PRURL      string
	MergedAt   time.Time
}
