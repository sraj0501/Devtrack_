package alerts

import "time"

// ReviewCommentEvent is emitted by the alert poller when a reviewer leaves a
// new comment on a PR authored by the developer. The infra layer stores this
// in pr_review_comments and then calls /review/classify.
type ReviewCommentEvent struct {
	Platform    string    // "github" | "azure" | "gitlab"
	Workspace   string
	PRID        string    // PR number or platform PR ID
	PRTitle     string
	CommentID   string    // platform-native comment ID (for idempotency)
	CommentBody string
	Reviewer    string
	CommentURL  string
	DetectedAt  time.Time
}
