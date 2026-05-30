package infra

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/gitsage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// EnhanceDeferredCommits enhances all pending deferred commits when the LLM is
// reachable, promoting each from "pending" to "enhanced" so it can be reviewed
// and applied via `devtrack commits review`. It is a no-op (0, nil) when the
// provider is unreachable, so it is safe to call on a schedule or pre-push.
func EnhanceDeferredCommits(database *db.Database) (int, error) {
	if database == nil {
		return 0, nil
	}
	if !gitsage.LLMReachable() {
		return 0, nil
	}

	pending, err := database.GetPendingDeferredCommits()
	if err != nil {
		return 0, err
	}

	enhanced := 0
	for _, c := range pending {
		msg, err := gitsage.EnhanceForDiff(c.OriginalMessage, c.DiffPatch, c.Branch)
		if err != nil || strings.TrimSpace(msg) == "" || msg == c.OriginalMessage {
			continue
		}
		if err := database.MarkDeferredCommitEnhanced(c.ID, msg); err != nil {
			log.Printf("⚠️  Could not mark deferred commit #%d enhanced: %v", c.ID, err)
			continue
		}
		enhanced++
	}
	return enhanced, nil
}

// deferredSnapshotRef is the ref that pins a deferred commit's snapshot object.
// It must match the format used when queuing (deferred_commit.go).
func deferredSnapshotRef(id int64) string {
	return fmt.Sprintf("refs/devtrack/deferred/%d", id)
}

// ExpireDeferredCommits marks pending/enhanced deferred commits older than
// expiryHours as expired, first pruning each one's pinned snapshot ref from its
// repository so no git objects are left dangling. Returns the number expired.
func ExpireDeferredCommits(database *db.Database, expiryHours int) (int, error) {
	if database == nil || expiryHours <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(expiryHours) * time.Hour)

	expiring, err := database.GetExpirableDeferredCommits(cutoff)
	if err != nil {
		return 0, err
	}
	for _, c := range expiring {
		if c.SnapshotSHA == "" {
			continue
		}
		cmd := exec.Command("git", "update-ref", "-d", deferredSnapshotRef(c.ID))
		if c.RepoPath != "" {
			cmd.Dir = c.RepoPath
		}
		_ = cmd.Run() // best-effort: a missing ref/repo is fine
	}

	res, err := database.Exec(`
		UPDATE deferred_commits
		SET status = 'expired', updated_at = ?
		WHERE status IN ('pending','enhanced') AND created_at < ?
	`, time.Now(), cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
