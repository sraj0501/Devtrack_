package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/infra"
)

// handleCommitQueue handles the internal commit-queue command (called by git wrapper)
func (cli *CLI) handleCommitQueue() error {
	// Parse flags
	message := ""
	branch := ""
	repoPath := ""
	filesStr := ""

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--message":
			if i+1 < len(os.Args) {
				message = os.Args[i+1]
				i++
			}
		case "--branch":
			if i+1 < len(os.Args) {
				branch = os.Args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(os.Args) {
				repoPath = os.Args[i+1]
				i++
			}
		case "--files":
			if i+1 < len(os.Args) {
				filesStr = os.Args[i+1]
				i++
			}
		}
	}

	if message == "" {
		return fmt.Errorf("--message is required")
	}

	// Read diff from stdin
	diffPatch := ""
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data := make([]byte, 0, 1024*1024) // 1MB max
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				data = append(data, buf[:n]...)
			}
			if err != nil {
				break
			}
			if len(data) > 1024*1024 {
				break // cap at 1MB
			}
		}
		diffPatch = string(data)
	}

	// Parse files
	var files []string
	if filesStr != "" {
		files = strings.Split(filesStr, ",")
	}

	// Open database and queue
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	mgr := NewDeferredCommitManager(db)
	id, err := mgr.QueueCommit(message, diffPatch, branch, repoPath, files)
	if err != nil {
		return err
	}

	fmt.Printf("Commit queued (ID: %d)\n", id)
	return nil
}

// handleCommits handles commits subcommands (pending, review)
func (cli *CLI) handleCommits() error {
	subCmd := "pending"
	if len(os.Args) > 2 {
		subCmd = os.Args[2]
	}

	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	mgr := NewDeferredCommitManager(db)

	switch subCmd {
	case "pending":
		return mgr.ListPending()
	case "review":
		return mgr.ReviewEnhanced()
	case "enhance":
		// Retry AI enhancement of queued commits (used by the pre-push hook and
		// available manually). Never fails the caller — pushes must not be blocked.
		n, err := infra.EnhanceDeferredCommits(db)
		if err != nil {
			fmt.Printf("commits enhance: %v\n", err)
			return nil
		}
		if n > 0 {
			fmt.Printf("✨ Enhanced %d queued commit(s) — run 'devtrack commits review'\n", n)
		}
		return nil
	default:
		fmt.Printf("Unknown commits subcommand: %s\n", subCmd)
		fmt.Println("Usage:")
		fmt.Println("  devtrack commits pending  - List deferred commits")
		fmt.Println("  devtrack commits review   - Review enhanced commits")
		fmt.Println("  devtrack commits enhance  - Retry AI enhancement of queued commits")
		return nil
	}
}
