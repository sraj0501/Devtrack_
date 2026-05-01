package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitMonitor handles Git repository monitoring and commit detection
type GitMonitor struct {
	repoPath string
	repo     *git.Repository
	watcher  *fsnotify.Watcher
	stopChan chan bool
}

// CommitInfo contains information about a detected commit
type CommitInfo struct {
	Hash      string
	Message   string
	Author    string
	Timestamp time.Time
	Files     []string
}

// NewGitMonitor creates a new GitMonitor instance
func NewGitMonitor(repoPath string) (*GitMonitor, error) {
	// Open the repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	// Create file system watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &GitMonitor{
		repoPath: repoPath,
		repo:     repo,
		watcher:  watcher,
		stopChan: make(chan bool),
	}, nil
}

// Start begins monitoring the Git repository for commits
func (gm *GitMonitor) Start(onCommit func(CommitInfo)) error {
	// Watch the .git directory for changes
	gitDir := filepath.Join(gm.repoPath, ".git")
	if err := gm.watcher.Add(gitDir); err != nil {
		return fmt.Errorf("failed to watch .git directory: %w", err)
	}

	// Watch the HEAD file specifically
	headFile := filepath.Join(gitDir, "HEAD")
	if err := gm.watcher.Add(headFile); err != nil {
		log.Printf("Warning: failed to watch HEAD file: %v", err)
	}

	// Watch the refs directory where branch pointers are updated on commits
	refsDir := filepath.Join(gitDir, "refs", "heads")
	if err := gm.watcher.Add(refsDir); err != nil {
		log.Printf("Warning: failed to watch refs/heads directory: %v", err)
	}

	// Watch COMMIT_EDITMSG which is updated on every commit
	commitMsgFile := filepath.Join(gitDir, "COMMIT_EDITMSG")
	if err := gm.watcher.Add(commitMsgFile); err != nil {
		log.Printf("Warning: failed to watch COMMIT_EDITMSG file: %v", err)
	}

	log.Printf("Started monitoring Git repository: %s", gm.repoPath)

	// Store the last commit hash to detect new commits
	lastCommit, err := gm.getLatestCommit()
	if err != nil {
		log.Printf("Warning: could not get initial commit: %v", err)
	}

	go func() {
		// Add polling ticker as fallback for Docker volume mounts where fsnotify may not work
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Periodic check for new commits (works reliably across Docker volumes)
				currentCommit, err := gm.getLatestCommit()
				if err != nil {
					log.Printf("Error getting latest commit: %v", err)
					continue
				}

				// If we have a new commit, trigger the callback
				if lastCommit == nil || currentCommit.Hash != lastCommit.Hash {
					log.Printf("New commit detected: %s - %s", currentCommit.Hash[:8], currentCommit.Message)
					onCommit(*currentCommit)
					lastCommit = currentCommit
				}

			case event, ok := <-gm.watcher.Events:
				if !ok {
					return
				}

				// Check if this is a relevant git event
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					// Skip lock files and temporary files
					if strings.Contains(event.Name, ".lock") || strings.Contains(event.Name, "~") {
						continue
					}

					// Small delay to allow git operations to complete
					time.Sleep(100 * time.Millisecond)

					// Check for new commit
					currentCommit, err := gm.getLatestCommit()
					if err != nil {
						log.Printf("Error getting latest commit: %v", err)
						continue
					}

					// If we have a new commit, trigger the callback
					if lastCommit == nil || currentCommit.Hash != lastCommit.Hash {
						log.Printf("New commit detected (fsnotify): %s", currentCommit.Hash[:8])
						onCommit(*currentCommit)
						lastCommit = currentCommit
					}
				}

			case err, ok := <-gm.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)

			case <-gm.stopChan:
				log.Println("Stopping Git monitor")
				return
			}
		}
	}()

	return nil
}

// Stop stops the Git monitoring
func (gm *GitMonitor) Stop() {
	close(gm.stopChan)
	if gm.watcher != nil {
		gm.watcher.Close()
	}
}

// getLatestCommit retrieves the most recent commit information
func (gm *GitMonitor) getLatestCommit() (*CommitInfo, error) {
	ref, err := gm.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := gm.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	// Get the files changed in this commit
	files, err := gm.getChangedFiles(commit)
	if err != nil {
		log.Printf("Warning: could not get changed files: %v", err)
		files = []string{}
	}

	return &CommitInfo{
		Hash:      commit.Hash.String(),
		Message:   strings.TrimSpace(commit.Message),
		Author:    commit.Author.Name,
		Timestamp: commit.Author.When,
		Files:     files,
	}, nil
}

// getChangedFiles returns the list of files changed in a commit
func (gm *GitMonitor) getChangedFiles(commit *object.Commit) ([]string, error) {
	var files []string

	// Get the tree for this commit
	tree, err := commit.Tree()
	if err != nil {
		return files, err
	}

	// If this is the first commit, list all files
	if commit.NumParents() == 0 {
		err = tree.Files().ForEach(func(f *object.File) error {
			files = append(files, f.Name)
			return nil
		})
		return files, err
	}

	// Get parent commit
	parent, err := commit.Parent(0)
	if err != nil {
		return files, err
	}

	parentTree, err := parent.Tree()
	if err != nil {
		return files, err
	}

	// Compare trees to find changes
	changes, err := parentTree.Diff(tree)
	if err != nil {
		return files, err
	}

	for _, change := range changes {
		from, to, err := change.Files()
		if err != nil {
			continue
		}

		if from != nil {
			files = append(files, from.Name)
		}
		if to != nil && (from == nil || from.Name != to.Name) {
			files = append(files, to.Name)
		}
	}

	return files, nil
}

// InstallPostCommitHook installs a post-commit hook to trigger the daemon
func InstallPostCommitHook(repoPath string) error {
	hookPath := filepath.Join(repoPath, ".git", "hooks", "post-commit")

	// Check if hook already exists
	if _, err := os.Stat(hookPath); err == nil {
		log.Printf("Post-commit hook already exists at: %s", hookPath)
		return nil
	}

	// Create the hook script with dynamic log path
	commitLogPath := filepath.Join(GetDevTrackDir(), "commit.log")
	hookContent := fmt.Sprintf(`#!/bin/sh
# Auto-generated by devtrack - Git commit detection hook
# This hook notifies the devtrack daemon about new commits

# Notify the daemon
echo "Commit detected at $(date)" >> %s

exit 0
`, commitLogPath)

	// Write the hook file
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("failed to create post-commit hook: %w", err)
	}

	log.Printf("✓ Installed post-commit hook at: %s", hookPath)
	return nil
}

// IsGitRepository checks if a directory is a Git repository
func IsGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}
