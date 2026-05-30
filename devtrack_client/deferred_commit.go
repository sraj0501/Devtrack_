package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/infra"
)

// DeferredCommitManager handles commits that were queued for later AI enhancement
type DeferredCommitManager struct {
	db *Database
}

// NewDeferredCommitManager creates a new deferred commit manager
func NewDeferredCommitManager(db *Database) *DeferredCommitManager {
	return &DeferredCommitManager{db: db}
}

// QueueCommit stores a commit for later AI enhancement. In addition to the
// text patch it captures the staged content as a durable, content-addressed git
// object (a dangling commit pinned by refs/devtrack/deferred/<id>). That object
// is immune to later working-tree changes and garbage collection, so the queued
// work can always be recovered and applied via a 3-way merge — fixing the
// fragility of replaying a raw text patch.
func (dcm *DeferredCommitManager) QueueCommit(message, diffPatch, branch, repoPath string, filesChanged []string) (int64, error) {
	filesJSON, err := json.Marshal(filesChanged)
	if err != nil {
		filesJSON = []byte("[]")
	}

	// Capture a durable snapshot of the staged tree (best-effort: if the git
	// plumbing fails we still queue the text patch).
	baseSHA, snapshotSHA := captureSnapshot(repoPath, message)

	record := DeferredCommitRecord{
		OriginalMessage: message,
		DiffPatch:       diffPatch,
		Branch:          branch,
		RepoPath:        repoPath,
		FilesChanged:    string(filesJSON),
		Status:          "pending",
		BaseSHA:         baseSHA,
		SnapshotSHA:     snapshotSHA,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	id, err := dcm.db.InsertDeferredCommit(record)
	if err != nil {
		return 0, fmt.Errorf("failed to queue deferred commit: %w", err)
	}

	// Pin the snapshot under a ref so it survives GC and tree changes.
	if snapshotSHA != "" {
		if _, refErr := gitOut(repoPath, "update-ref", snapshotRef(id), snapshotSHA); refErr != nil {
			log.Printf("Deferred commit %d: could not pin snapshot ref: %v", id, refErr)
		}
	}

	log.Printf("Deferred commit queued (id=%d, branch=%s, snapshot=%s)", id, branch, shortSHA(snapshotSHA))
	return id, nil
}

// captureSnapshot writes the current staged index to a tree and creates a
// dangling commit object from it. Returns (baseSHA, snapshotSHA); either may be
// empty if git plumbing is unavailable (e.g. an empty repository).
func captureSnapshot(repoPath, message string) (baseSHA, snapshotSHA string) {
	baseSHA, _ = gitOut(repoPath, "rev-parse", "HEAD")
	tree, err := gitOut(repoPath, "write-tree")
	if err != nil || tree == "" {
		return baseSHA, ""
	}
	args := []string{"commit-tree", tree}
	if baseSHA != "" {
		args = append(args, "-p", baseSHA)
	}
	args = append(args, "-m", message)
	snapshotSHA, err = gitOut(repoPath, args...)
	if err != nil {
		return baseSHA, ""
	}
	return baseSHA, snapshotSHA
}

// snapshotRef is the ref that pins a deferred commit's snapshot object.
func snapshotRef(id int64) string {
	return fmt.Sprintf("refs/devtrack/deferred/%d", id)
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// gitOut runs a git command in repoPath and returns trimmed stdout.
func gitOut(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// ListPending shows all pending and enhanced deferred commits
func (dcm *DeferredCommitManager) ListPending() error {
	pending, err := dcm.db.GetPendingDeferredCommits()
	if err != nil {
		return fmt.Errorf("failed to get pending commits: %w", err)
	}

	enhanced, err := dcm.db.GetEnhancedDeferredCommits()
	if err != nil {
		return fmt.Errorf("failed to get enhanced commits: %w", err)
	}

	if len(pending) == 0 && len(enhanced) == 0 {
		fmt.Println("No deferred commits.")
		return nil
	}

	if len(enhanced) > 0 {
		fmt.Printf("\n\033[32m● Ready for review (%d):\033[0m\n", len(enhanced))
		fmt.Println(strings.Repeat("─", 60))
		for _, c := range enhanced {
			fmt.Printf("  ID: %d  Branch: %s  Created: %s\n", c.ID, c.Branch, c.CreatedAt.Format("2006-01-02 15:04"))
			fmt.Printf("  Original:  %s\n", firstLine(c.OriginalMessage))
			fmt.Printf("  Enhanced:  %s\n", firstLine(c.EnhancedMessage))
			fmt.Println()
		}
	}

	if len(pending) > 0 {
		fmt.Printf("\n\033[33m● Awaiting AI enhancement (%d):\033[0m\n", len(pending))
		fmt.Println(strings.Repeat("─", 60))
		for _, c := range pending {
			fmt.Printf("  ID: %d  Branch: %s  Created: %s\n", c.ID, c.Branch, c.CreatedAt.Format("2006-01-02 15:04"))
			fmt.Printf("  Message:   %s\n", firstLine(c.OriginalMessage))
			fmt.Println()
		}
	}

	return nil
}

// ReviewEnhanced interactively reviews enhanced commits, letting user approve/reject
func (dcm *DeferredCommitManager) ReviewEnhanced() error {
	enhanced, err := dcm.db.GetEnhancedDeferredCommits()
	if err != nil {
		return fmt.Errorf("failed to get enhanced commits: %w", err)
	}

	if len(enhanced) == 0 {
		fmt.Println("No enhanced commits ready for review.")

		// Check pending
		pending, _, _, _, _ := dcm.db.GetDeferredCommitStats()
		if pending > 0 {
			fmt.Printf("\n%d commits still awaiting AI enhancement.\n", pending)
		}
		return nil
	}

	fmt.Printf("\n\033[34m🔍 Reviewing %d enhanced commit(s)\033[0m\n\n", len(enhanced))

	for _, c := range enhanced {
		fmt.Println(strings.Repeat("━", 60))
		fmt.Printf("Commit #%d  Branch: %s  Repo: %s\n", c.ID, c.Branch, c.RepoPath)
		fmt.Println(strings.Repeat("━", 60))
		fmt.Printf("\n\033[33mOriginal:\033[0m\n  %s\n", c.OriginalMessage)
		fmt.Printf("\n\033[32mEnhanced:\033[0m\n  %s\n", c.EnhancedMessage)
		fmt.Println()

		// Parse files
		var files []string
		json.Unmarshal([]byte(c.FilesChanged), &files)
		if len(files) > 0 {
			fmt.Printf("Files: %s\n", strings.Join(files, ", "))
		}
		fmt.Println()

		fmt.Print("\033[34m[A]ccept enhanced  [O]riginal message  [S]kip  [D]iscard: \033[0m")
		var choice string
		fmt.Scanln(&choice)

		switch strings.ToLower(choice) {
		case "a":
			if err := dcm.executeCommit(c, c.EnhancedMessage); err != nil {
				fmt.Printf("\033[31m✗ Commit failed: %v\033[0m\n", err)
			} else {
				dcm.db.MarkDeferredCommitCommitted(c.ID)
				fmt.Println("\033[32m✓ Committed with enhanced message\033[0m")
			}
		case "o":
			if err := dcm.executeCommit(c, c.OriginalMessage); err != nil {
				fmt.Printf("\033[31m✗ Commit failed: %v\033[0m\n", err)
			} else {
				dcm.db.MarkDeferredCommitCommitted(c.ID)
				fmt.Println("\033[32m✓ Committed with original message\033[0m")
			}
		case "d":
			dcm.db.MarkDeferredCommitExpired(c.ID)
			dcm.cleanupSnapshot(c.RepoPath, c.ID)
			fmt.Println("\033[33m✗ Commit discarded\033[0m")
		default:
			fmt.Println("Skipped")
		}
		fmt.Println()
	}

	return nil
}

// executeCommit applies the queued change and commits it with the given message.
// It is resilient to a working tree that moved since the change was queued:
//
//  1. Fast path — nothing moved and the snapshot is still exactly staged: just
//     commit the staged content.
//  2. Already applied — the change is already in the tree: nothing to do.
//  3. General path — 3-way apply the patch. The pinned snapshot object supplies
//     the blobs the merge needs, so this succeeds even when context drifted.
//  4. On a genuine conflict, surface a clean manual recovery using the snapshot
//     commit (which is never lost), instead of failing opaquely.
func (dcm *DeferredCommitManager) executeCommit(record DeferredCommitRecord, message string) error {
	repoPath := record.RepoPath
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine repo path: %w", err)
		}
	}

	head, _ := gitOut(repoPath, "rev-parse", "HEAD")

	// (1) Fast path: base unchanged and the snapshot is still staged verbatim.
	if record.SnapshotSHA != "" && record.BaseSHA != "" && head == record.BaseSHA {
		staged, _ := gitOut(repoPath, "write-tree")
		snapTree, _ := gitOut(repoPath, "rev-parse", record.SnapshotSHA+"^{tree}")
		if staged != "" && staged == snapTree {
			if err := gitCommitMessage(repoPath, message); err != nil {
				return err
			}
			dcm.cleanupSnapshot(repoPath, record.ID)
			return nil
		}
	}

	// (2) Already applied? If the patch reverse-applies cleanly, the change is
	// already present — don't duplicate it.
	if record.DiffPatch != "" && patchAlreadyApplied(repoPath, record.DiffPatch) {
		dcm.cleanupSnapshot(repoPath, record.ID)
		return fmt.Errorf("these changes are already present in the working tree — nothing to apply")
	}

	// (3) General path: 3-way apply (pinned snapshot blobs make this robust).
	if record.DiffPatch != "" {
		if err := gitApply3way(repoPath, record.DiffPatch); err != nil {
			// (4) Conflict — the snapshot commit is intact; offer manual recovery.
			recovery := record.SnapshotSHA
			if recovery == "" {
				recovery = snapshotRef(record.ID)
			}
			return fmt.Errorf("could not auto-apply — the working tree moved: %w\n"+
				"  Recover manually:  git cherry-pick %s\n"+
				"  (resolve conflicts, reword with the enhanced message, then 'devtrack commits review')",
				err, recovery)
		}
	}

	if err := gitCommitMessage(repoPath, message); err != nil {
		return err
	}
	dcm.cleanupSnapshot(repoPath, record.ID)
	return nil
}

// gitCommitMessage commits the currently staged changes with the given message.
func gitCommitMessage(repoPath, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w\n%s", err, string(out))
	}
	return nil
}

// gitApply3way applies a patch with a real 3-way merge (falling back to conflict
// markers instead of a hard failure when context has drifted).
func gitApply3way(repoPath, patch string) error {
	cmd := exec.Command("git", "apply", "--3way", "--index", "-")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader(patch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// patchAlreadyApplied reports whether a patch's changes are already in the tree
// (it reverse-applies cleanly).
func patchAlreadyApplied(repoPath, patch string) bool {
	cmd := exec.Command("git", "apply", "--reverse", "--check", "-")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader(patch)
	return cmd.Run() == nil
}

// cleanupSnapshot removes the pinned snapshot ref once a deferred commit is
// resolved (best-effort).
func (dcm *DeferredCommitManager) cleanupSnapshot(repoPath string, id int64) {
	_, _ = gitOut(repoPath, "update-ref", "-d", snapshotRef(id))
}

// ExpireOldCommits marks old pending/enhanced commits as expired, pruning each
// one's pinned snapshot ref. Delegates to the shared infra implementation so the
// daemon scheduler and the CLI behave identically.
func (dcm *DeferredCommitManager) ExpireOldCommits() (int, error) {
	expiryHours := GetDeferredCommitExpiryHours()
	count, err := infra.ExpireDeferredCommits(dcm.db, expiryHours)
	if err != nil {
		return 0, fmt.Errorf("failed to expire old commits: %w", err)
	}
	if count > 0 {
		log.Printf("Expired %d old deferred commits (older than %dh); pruned their snapshot refs", count, expiryHours)
	}
	return count, nil
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
