package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DeferredCommitManager handles commits that were queued for later AI enhancement
type DeferredCommitManager struct {
	db *Database
}

// NewDeferredCommitManager creates a new deferred commit manager
func NewDeferredCommitManager(db *Database) *DeferredCommitManager {
	return &DeferredCommitManager{db: db}
}

// QueueCommit stores a commit for later AI enhancement.
// Called by `devtrack commit-queue` from the git wrapper.
func (dcm *DeferredCommitManager) QueueCommit(message, diffPatch, branch, repoPath string, filesChanged []string) (int64, error) {
	filesJSON, err := json.Marshal(filesChanged)
	if err != nil {
		filesJSON = []byte("[]")
	}

	record := DeferredCommitRecord{
		OriginalMessage: message,
		DiffPatch:       diffPatch,
		Branch:          branch,
		RepoPath:        repoPath,
		FilesChanged:    string(filesJSON),
		Status:          "pending",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	id, err := dcm.db.InsertDeferredCommit(record)
	if err != nil {
		return 0, fmt.Errorf("failed to queue deferred commit: %w", err)
	}

	log.Printf("Deferred commit queued (id=%d, branch=%s)", id, branch)
	return id, nil
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
			fmt.Println("\033[33m✗ Commit discarded\033[0m")
		default:
			fmt.Println("Skipped")
		}
		fmt.Println()
	}

	return nil
}

// executeCommit runs git commit with the stored diff applied
func (dcm *DeferredCommitManager) executeCommit(record DeferredCommitRecord, message string) error {
	repoPath := record.RepoPath
	if repoPath == "" {
		var err error
		repoPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine repo path: %w", err)
		}
	}

	// Apply the diff patch to stage changes
	if record.DiffPatch != "" {
		applyCmd := exec.Command("git", "apply", "--cached", "-")
		applyCmd.Dir = repoPath
		applyCmd.Stdin = strings.NewReader(record.DiffPatch)
		if output, err := applyCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to apply patch: %w\n%s", err, string(output))
		}
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = repoPath
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w\n%s", err, string(output))
	}

	return nil
}

// ExpireOldCommits marks old pending/enhanced commits as expired
func (dcm *DeferredCommitManager) ExpireOldCommits() (int, error) {
	expiryHours := GetDeferredCommitExpiryHours()
	cutoff := time.Now().Add(-time.Duration(expiryHours) * time.Hour)

	// This would need a new DB method; for now use inline query
	result, err := dcm.db.db.Exec(`
		UPDATE deferred_commits
		SET status = 'expired', updated_at = ?
		WHERE status IN ('pending', 'enhanced') AND created_at < ?
	`, time.Now(), cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to expire old commits: %w", err)
	}

	count, _ := result.RowsAffected()
	if count > 0 {
		log.Printf("Expired %d old deferred commits (older than %dh)", count, expiryHours)
	}
	return int(count), nil
}

// deferredCommitStats wraps the stats from DB for convenience
type deferredCommitStats struct {
	pending   int
	enhanced  int
	committed int
	expired   int
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
