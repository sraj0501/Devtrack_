package reviewer

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// newLoopTestDB creates a fully functional SQLite DB using NewDatabaseAtPath,
// which applies the base schema plus all migration-managed tables (including
// pr_review_comments with attempt_count from migrations 012 and 013).
func newLoopTestDB(t *testing.T) *db.Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "loop_test.db")
	database, err := db.NewDatabaseAtPath(dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseAtPath: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// insertFixableComment inserts a pre-classified auto_fixable comment into the test DB.
func insertFixableComment(t *testing.T, database *db.Database, platform, commentID, prID string) {
	t.Helper()
	c := db.PRReviewComment{
		Platform:     platform,
		CommentID:    commentID,
		PRID:         prID,
		Workspace:    "test-ws",
		Status:       "classified",
		CommentBody:  fmt.Sprintf("Fix the formatting in comment %s", commentID),
		ClassifiedAs: "auto_fixable",
		FixHint:      "run gofmt",
	}
	if err := database.InsertPRReviewComment(c); err != nil {
		t.Fatalf("InsertPRReviewComment(%s): %v", commentID, err)
	}
}

// mockChecker is a PRApprovalChecker that returns a configurable approved state.
type mockChecker struct {
	approved  bool
	callCount int
}

func (m *mockChecker) IsPRApproved(prID, workspace string) (bool, error) {
	m.callCount++
	return m.approved, nil
}

// successCmdBuilder returns an agent cmdBuilder that always exits 0 (success).
func successCmdBuilder(t *testing.T) cmdBuilderFunc {
	return mockCmdBuilder(t, "success")
}

// failCmdBuilder returns an agent cmdBuilder that always exits non-zero (failure).
func failCmdBuilder(t *testing.T) cmdBuilderFunc {
	return mockCmdBuilder(t, "nonzero")
}

// TestPRFixLoopHappyPath verifies that a single auto_fixable comment is fixed,
// pushed (push fails gracefully since repoPath is empty), and the PR is then
// reported as approved by the checker.
func TestPRFixLoopHappyPath(t *testing.T) {
	t.Setenv("REVIEW_POLL_INTERVAL_SECS", "1")

	database := newLoopTestDB(t)
	insertFixableComment(t, database, "github", "cmt-001", "pr-10")

	ag := &Agent{
		backend:    BackendClaudeCode,
		timeoutSec: 10,
		cmdBuilder: successCmdBuilder(t),
	}
	checker := &mockChecker{approved: true}

	loop := NewPRFixLoop(database, ag, checker)
	report := loop.Run(context.Background(), "github", "pr-10", "test-ws", "" /* repoPath */)

	if report.Stuck {
		t.Errorf("expected Stuck=false (PR approved), got Stuck=true; BlockerReason=%q", report.BlockerReason)
	}
	if checker.callCount < 1 {
		t.Errorf("expected IsPRApproved to be called at least once, callCount=%d", checker.callCount)
	}

	// Verify the comment was marked fix_applied.
	got, err := database.GetPRReviewComment("github", "cmt-001")
	if err != nil {
		t.Fatalf("GetPRReviewComment: %v", err)
	}
	if got == nil {
		t.Fatal("GetPRReviewComment returned nil")
	}
	if got.Status != "fix_applied" {
		t.Errorf("expected status=fix_applied, got %q", got.Status)
	}
}

// TestPRFixLoopStuckPath verifies that when the agent fails twice on the same
// comment, the loop returns EscalationReport{Stuck: true}.
func TestPRFixLoopStuckPath(t *testing.T) {
	t.Setenv("REVIEW_POLL_INTERVAL_SECS", "1")

	database := newLoopTestDB(t)
	insertFixableComment(t, database, "github", "cmt-002", "pr-20")

	ag := &Agent{
		backend:    BackendClaudeCode,
		timeoutSec: 10,
		cmdBuilder: failCmdBuilder(t),
	}
	checker := &mockChecker{approved: false}

	loop := NewPRFixLoop(database, ag, checker)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report := loop.Run(ctx, "github", "pr-20", "test-ws", "")

	if !report.Stuck {
		t.Errorf("expected Stuck=true, got Stuck=false")
	}
	if report.BlockerReason == "" {
		t.Error("expected non-empty BlockerReason for stuck report")
	}
	t.Logf("BlockerReason=%q", report.BlockerReason)

	// Verify attempt_count reached MaxAttemptsPerComment.
	got, err := database.GetPRReviewComment("github", "cmt-002")
	if err != nil {
		t.Fatalf("GetPRReviewComment: %v", err)
	}
	if got == nil {
		t.Fatal("GetPRReviewComment returned nil")
	}
	if got.AttemptCount < MaxAttemptsPerComment {
		t.Errorf("expected AttemptCount >= %d, got %d", MaxAttemptsPerComment, got.AttemptCount)
	}
}

// TestPRFixLoopMaxAttempts verifies that when the agent fails on 5 different
// comments (MaxAttemptsPerPR), the loop returns Stuck=true with
// BlockerReason="max PR attempts reached".
func TestPRFixLoopMaxAttempts(t *testing.T) {
	t.Setenv("REVIEW_POLL_INTERVAL_SECS", "1")

	database := newLoopTestDB(t)
	// Insert MaxAttemptsPerPR fixable comments — each will fail once before the
	// per-PR guard fires on the second pass through the first comment.
	for i := range MaxAttemptsPerPR {
		insertFixableComment(t, database, "github", fmt.Sprintf("cmt-pr5-%d", i), "pr-30")
	}

	ag := &Agent{
		backend:    BackendClaudeCode,
		timeoutSec: 10,
		cmdBuilder: failCmdBuilder(t),
	}
	checker := &mockChecker{approved: false}

	loop := NewPRFixLoop(database, ag, checker)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report := loop.Run(ctx, "github", "pr-30", "test-ws", "")

	if !report.Stuck {
		t.Errorf("expected Stuck=true, got Stuck=false")
	}
	if report.BlockerReason != "max PR attempts reached" {
		t.Errorf("expected BlockerReason=%q, got %q", "max PR attempts reached", report.BlockerReason)
	}
}
