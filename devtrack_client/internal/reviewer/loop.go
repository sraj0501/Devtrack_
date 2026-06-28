package reviewer

import (
	"context"
	"log"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

const MaxAttemptsPerComment = 2
const MaxAttemptsPerPR = 5

// EscalationReport is the outcome of a PRFixLoop.Run call.
type EscalationReport struct {
	PRTitle       string
	BlockerReason string // human-readable: "Agent failed twice on comment <id>: <error>"
	CommentURL    string
	Stuck         bool // false = PR approved
}

// PRApprovalChecker is implemented by platform alerters that can check PR approval state.
type PRApprovalChecker interface {
	IsPRApproved(prID, workspace string) (bool, error)
}

// PRFixLoop orchestrates the fix-commit-push loop for a single PR.
type PRFixLoop struct {
	db      *db.Database
	agent   *Agent
	checker PRApprovalChecker
}

// NewPRFixLoop creates a PRFixLoop.
func NewPRFixLoop(database *db.Database, agent *Agent, checker PRApprovalChecker) *PRFixLoop {
	return &PRFixLoop{
		db:      database,
		agent:   agent,
		checker: checker,
	}
}

// Run processes all auto_fixable comments for the given PR in sequence.
// Blocks until the PR is approved, stuck, or ctx is cancelled.
func (l *PRFixLoop) Run(ctx context.Context, platform, prID, workspace, repoPath string) EscalationReport {
	attempts := 0
	pollInterval := time.Duration(config.GetReviewPollIntervalSecs()) * time.Second

	for {
		// Check for context cancellation.
		select {
		case <-ctx.Done():
			return EscalationReport{Stuck: true, BlockerReason: "context cancelled"}
		default:
		}

		// Load all classified comments for this PR.
		allComments, err := l.db.ListPRReviewCommentsByPR(platform, prID)
		if err != nil {
			log.Printf("review/loop: list comments for PR %s: %v", prID, err)
			return EscalationReport{Stuck: true, BlockerReason: "db error: " + err.Error()}
		}

		// Filter to auto_fixable + classified status.
		var fixable []db.PRReviewComment
		for _, c := range allComments {
			if c.ClassifiedAs == "auto_fixable" && c.Status == "classified" {
				fixable = append(fixable, c)
			}
		}

		if len(fixable) == 0 {
			// No pending fixable comments — check if PR is approved.
			if l.checker == nil {
				log.Printf("review/loop: no approval checker configured — PR %s waiting for manual check", prID)
			} else {
				approved, err := l.checker.IsPRApproved(prID, workspace)
				if err != nil {
					log.Printf("review/loop: IsPRApproved PR %s: %v", prID, err)
				}
				if approved {
					log.Printf("review: PR %s approved", prID)
					return EscalationReport{Stuck: false}
				}
			}
			// Still open — wait and re-poll.
			log.Printf("review/loop: PR %s still open, waiting %v before re-poll", prID, pollInterval)
			select {
			case <-ctx.Done():
				return EscalationReport{Stuck: true, BlockerReason: "context cancelled"}
			case <-time.After(pollInterval):
			}
			continue
		}

		// Process each fixable comment in order.
		for _, comment := range fixable {
			// Read current attempt_count directly from DB (may have been updated in a prior iteration).
			current, err := l.db.GetPRReviewComment(platform, comment.CommentID)
			if err != nil || current == nil {
				log.Printf("review/loop: get comment %s: %v", comment.CommentID, err)
				continue
			}
			if current.AttemptCount >= MaxAttemptsPerComment {
				return EscalationReport{
					Stuck:         true,
					BlockerReason: "agent failed twice on comment " + comment.CommentID,
					CommentURL:    comment.CommentURL,
				}
			}
			if attempts >= MaxAttemptsPerPR {
				return EscalationReport{
					Stuck:         true,
					BlockerReason: "max PR attempts reached",
				}
			}

			inv := AgentInvocation{
				RepoPath:    repoPath,
				CommentBody: comment.CommentBody,
				FixHint:     comment.FixHint,
				Backend:     BackendClaudeCode,
			}
			result := l.agent.Apply(ctx, inv)
			attempts++

			if result.Success {
				// Push the fix.
				if err := PushToRemote(ctx, repoPath, "HEAD"); err != nil {
					log.Printf("review/loop: push after fix for comment %s: %v", comment.CommentID, err)
					// Don't increment attempt_count — the fix was applied, push failed.
					// Still mark fix_applied so we don't re-apply; the developer must push manually.
				}
				_ = l.db.UpdatePRReviewCommentStatus(platform, comment.CommentID, "fix_applied", comment.ClassifiedAs, comment.FixHint)
				log.Printf("review/loop: fix applied for comment %s on PR %s (commit %s)", comment.CommentID, prID, result.CommitHash)
				// Restart the loop with a refreshed comment list.
				break
			}

			// Agent failed — increment attempt_count.
			newCount, incErr := l.db.IncrementPRReviewCommentAttempts(platform, comment.CommentID)
			if incErr != nil {
				log.Printf("review/loop: increment attempts for comment %s: %v", comment.CommentID, incErr)
			}
			log.Printf("review/loop: agent failed on comment %s (attempt %d/%d): %s",
				comment.CommentID, newCount, MaxAttemptsPerComment, result.Error)

			if newCount >= MaxAttemptsPerComment {
				return EscalationReport{
					Stuck:         true,
					BlockerReason: result.Error,
					CommentURL:    comment.CommentURL,
				}
			}
			// Try the next comment, or re-loop.
		}
	}
}
