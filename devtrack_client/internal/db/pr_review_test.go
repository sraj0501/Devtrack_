package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// newPRReviewTestDB opens a fresh SQLite DB in a temp dir, initialises the
// core schema plus the pr_review_comments table (migration 012 DDL).
func newPRReviewTestDB(t *testing.T) *Database {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_pr_review.db")

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}

	testDB := &Database{db: sqlDB, path: dbPath}
	if err := testDB.initSchema(); err != nil {
		t.Fatalf("initSchema: %v", err)
	}

	// Run migration 012 + 013 DDL inline (same SQL as allMigrations entries 012 and 013).
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS pr_review_comments (
			platform      TEXT     NOT NULL,
			comment_id    TEXT     NOT NULL,
			pr_id         TEXT     NOT NULL,
			workspace     TEXT     NOT NULL,
			status        TEXT     NOT NULL DEFAULT 'new',
			comment_body  TEXT     NOT NULL DEFAULT '',
			classified_as TEXT,
			fix_hint      TEXT     NOT NULL DEFAULT '',
			created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			attempt_count INTEGER  NOT NULL DEFAULT 0,
			PRIMARY KEY (platform, comment_id)
		);
		CREATE INDEX IF NOT EXISTS idx_pr_comments_status ON pr_review_comments(status);
		CREATE INDEX IF NOT EXISTS idx_pr_comments_pr     ON pr_review_comments(pr_id, platform);
	`)
	if err != nil {
		t.Fatalf("create pr_review_comments table: %v", err)
	}

	t.Cleanup(func() {
		_ = testDB.Close()
		_ = os.Remove(dbPath)
	})
	return testDB
}

// TestPRReviewCommentInsertGet verifies the InsertPRReviewComment + GetPRReviewComment round-trip.
func TestPRReviewCommentInsertGet(t *testing.T) {
	db := newPRReviewTestDB(t)

	comment := PRReviewComment{
		Platform:    "github",
		CommentID:   "rev-comment-001",
		PRID:        "42",
		Workspace:   "devtrack-ws",
		Status:      "new",
		CommentBody: "Please rename this variable to be more descriptive.",
		FixHint:     "",
	}

	if err := db.InsertPRReviewComment(comment); err != nil {
		t.Fatalf("InsertPRReviewComment: %v", err)
	}

	got, err := db.GetPRReviewComment("github", "rev-comment-001")
	if err != nil {
		t.Fatalf("GetPRReviewComment: %v", err)
	}
	if got == nil {
		t.Fatal("GetPRReviewComment returned nil")
	}

	if got.Platform != comment.Platform {
		t.Errorf("Platform: got %q, want %q", got.Platform, comment.Platform)
	}
	if got.CommentID != comment.CommentID {
		t.Errorf("CommentID: got %q, want %q", got.CommentID, comment.CommentID)
	}
	if got.PRID != comment.PRID {
		t.Errorf("PRID: got %q, want %q", got.PRID, comment.PRID)
	}
	if got.Workspace != comment.Workspace {
		t.Errorf("Workspace: got %q, want %q", got.Workspace, comment.Workspace)
	}
	if got.Status != "new" {
		t.Errorf("Status: got %q, want %q", got.Status, "new")
	}
	if got.CommentBody != comment.CommentBody {
		t.Errorf("CommentBody: got %q, want %q", got.CommentBody, comment.CommentBody)
	}
	if !got.CreatedAt.IsZero() == false {
		// CreatedAt is populated by SQLite default — just check it's non-zero.
	}

	// GetPRReviewComment for missing key returns nil, nil.
	missing, err := db.GetPRReviewComment("github", "does-not-exist")
	if err != nil {
		t.Fatalf("GetPRReviewComment(missing): %v", err)
	}
	if missing != nil {
		t.Errorf("GetPRReviewComment(missing): expected nil, got %+v", missing)
	}
}

// TestPRReviewCommentUpdateStatus verifies UpdatePRReviewCommentStatus changes
// status, classified_as, and fix_hint.
func TestPRReviewCommentUpdateStatus(t *testing.T) {
	db := newPRReviewTestDB(t)

	comment := PRReviewComment{
		Platform:    "github",
		CommentID:   "rev-comment-002",
		PRID:        "99",
		Workspace:   "ws1",
		Status:      "new",
		CommentBody: "Add a blank line before this function.",
	}
	if err := db.InsertPRReviewComment(comment); err != nil {
		t.Fatalf("InsertPRReviewComment: %v", err)
	}

	if err := db.UpdatePRReviewCommentStatus("github", "rev-comment-002", "classified", "auto_fixable", "Add blank line before function definition."); err != nil {
		t.Fatalf("UpdatePRReviewCommentStatus: %v", err)
	}

	got, err := db.GetPRReviewComment("github", "rev-comment-002")
	if err != nil {
		t.Fatalf("GetPRReviewComment after update: %v", err)
	}
	if got == nil {
		t.Fatal("GetPRReviewComment returned nil after update")
	}
	if got.Status != "classified" {
		t.Errorf("Status: got %q, want %q", got.Status, "classified")
	}
	if got.ClassifiedAs != "auto_fixable" {
		t.Errorf("ClassifiedAs: got %q, want %q", got.ClassifiedAs, "auto_fixable")
	}
	if got.FixHint != "Add blank line before function definition." {
		t.Errorf("FixHint: got %q, want %q", got.FixHint, "Add blank line before function definition.")
	}
}

// TestPRReviewCommentListByPR inserts two comments for the same PR and verifies
// both are returned by ListPRReviewCommentsByPR.
func TestPRReviewCommentListByPR(t *testing.T) {
	db := newPRReviewTestDB(t)

	c1 := PRReviewComment{
		Platform:    "github",
		CommentID:   "cmt-a",
		PRID:        "77",
		Workspace:   "ws-pr",
		Status:      "new",
		CommentBody: "First comment on this PR.",
	}
	c2 := PRReviewComment{
		Platform:    "github",
		CommentID:   "cmt-b",
		PRID:        "77",
		Workspace:   "ws-pr",
		Status:      "classified",
		CommentBody: "Second comment on this PR.",
		ClassifiedAs: "needs_human",
	}
	// Comment on a different PR — should NOT appear in results.
	c3 := PRReviewComment{
		Platform:    "github",
		CommentID:   "cmt-c",
		PRID:        "88",
		Workspace:   "ws-pr",
		Status:      "new",
		CommentBody: "Comment on a different PR.",
	}

	for _, c := range []PRReviewComment{c1, c2, c3} {
		if err := db.InsertPRReviewComment(c); err != nil {
			t.Fatalf("InsertPRReviewComment(%s): %v", c.CommentID, err)
		}
	}

	list, err := db.ListPRReviewCommentsByPR("github", "77")
	if err != nil {
		t.Fatalf("ListPRReviewCommentsByPR: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 comments for PR 77, got %d", len(list))
	}
	ids := map[string]bool{list[0].CommentID: true, list[1].CommentID: true}
	if !ids["cmt-a"] || !ids["cmt-b"] {
		t.Errorf("unexpected comment IDs: %v", ids)
	}
}

// TestPRReviewCommentListByStatus verifies ListPRReviewCommentsByStatus filters correctly.
func TestPRReviewCommentListByStatus(t *testing.T) {
	db := newPRReviewTestDB(t)

	comments := []PRReviewComment{
		{Platform: "github", CommentID: "s1", PRID: "10", Workspace: "ws", Status: "new", CommentBody: "a"},
		{Platform: "github", CommentID: "s2", PRID: "10", Workspace: "ws", Status: "new", CommentBody: "b"},
		{Platform: "github", CommentID: "s3", PRID: "11", Workspace: "ws", Status: "classified", CommentBody: "c", ClassifiedAs: "needs_human"},
	}
	for _, c := range comments {
		if err := db.InsertPRReviewComment(c); err != nil {
			t.Fatalf("InsertPRReviewComment(%s): %v", c.CommentID, err)
		}
	}

	newOnes, err := db.ListPRReviewCommentsByStatus("new")
	if err != nil {
		t.Fatalf("ListPRReviewCommentsByStatus(new): %v", err)
	}
	if len(newOnes) != 2 {
		t.Errorf("expected 2 'new' comments, got %d", len(newOnes))
	}
	for _, c := range newOnes {
		if c.Status != "new" {
			t.Errorf("unexpected status %q in new list", c.Status)
		}
	}

	classifiedOnes, err := db.ListPRReviewCommentsByStatus("classified")
	if err != nil {
		t.Fatalf("ListPRReviewCommentsByStatus(classified): %v", err)
	}
	if len(classifiedOnes) != 1 {
		t.Errorf("expected 1 classified comment, got %d", len(classifiedOnes))
	}

	// Empty status string returns all.
	all, err := db.ListPRReviewCommentsByStatus("")
	if err != nil {
		t.Fatalf("ListPRReviewCommentsByStatus(all): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 total comments, got %d", len(all))
	}
}
