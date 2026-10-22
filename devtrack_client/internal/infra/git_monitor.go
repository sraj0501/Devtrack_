package infra

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// GitMonitor handles Git repository monitoring and commit detection
type GitMonitor struct {
	repoPath string
	repo     *gogit.Repository
	watcher  *fsnotify.Watcher
	stopChan chan bool
	seen     map[plumbing.Hash]bool // includes pre-monitor history and delivered commits
}

// CommitInfo contains information about a detected commit
type CommitInfo struct {
	Hash      string
	Message   string
	Author    string
	Timestamp time.Time
	Files     []string
	Branch    string // current branch name at commit time ("" if detached HEAD)
	IsMerge   bool   // true when the commit has 2+ parents (a merge commit)
}

// NewGitMonitor creates a new GitMonitor instance
func NewGitMonitor(repoPath string) (*GitMonitor, error) {
	// Open the repository
	repo, err := gogit.PlainOpen(repoPath)
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

	// Watch the refs directory where branch pointers are updated on commits.
	// This directory may not exist yet in a freshly-initialised repo with no commits.
	refsDir := filepath.Join(gitDir, "refs", "heads")
	if err := gm.watcher.Add(refsDir); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to watch refs/heads directory: %v", err)
	}

	// Watch COMMIT_EDITMSG which is updated on every commit.
	// It does not exist until the first commit is made.
	commitMsgFile := filepath.Join(gitDir, "COMMIT_EDITMSG")
	if err := gm.watcher.Add(commitMsgFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to watch COMMIT_EDITMSG file: %v", err)
	}

	// Determine initial state; an empty repo (no commits) is valid — monitoring continues.
	gm.seen = make(map[plumbing.Hash]bool)
	_, err := gm.repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			log.Printf("Started monitoring Git repository (no commits yet): %s", gm.repoPath)
		} else {
			return fmt.Errorf("read initial HEAD: %w", err)
		}
	} else {
		// Seed all existing refs so checking out an old branch does not import
		// its pre-monitor history as newly completed work.
		iter, walkErr := gm.repo.Log(&gogit.LogOptions{All: true})
		if walkErr != nil {
			return fmt.Errorf("read initial commit history: %w", walkErr)
		}
		if walkErr = iter.ForEach(func(c *object.Commit) error { gm.seen[c.Hash] = true; return nil }); walkErr != nil {
			return fmt.Errorf("read initial commit history: %w", walkErr)
		}
		log.Printf("Started monitoring Git repository: %s", gm.repoPath)
	}

	go func() {
		// Add polling ticker as fallback for Docker volume mounts where fsnotify may not work
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				gm.checkNewCommits(onCommit)

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

					gm.checkNewCommits(onCommit)
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

// checkNewCommits walks every unseen ancestor, oldest first. A slow callback
// cannot collapse a burst to the latest HEAD. Known ancestors prune the walk,
// including shared merge parents and branch checkouts already observed.
func (gm *GitMonitor) checkNewCommits(onCommit func(CommitInfo)) {
	head, err := gm.repo.Head()
	if err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			log.Printf("Read HEAD: %v", err)
		}
		return
	}
	if len(gm.seen) == 0 {
		gitDir := filepath.Join(gm.repoPath, ".git")
		_ = gm.watcher.Add(filepath.Join(gitDir, "refs", "heads"))
		_ = gm.watcher.Add(filepath.Join(gitDir, "COMMIT_EDITMSG"))
	}
	// A callback can block on a downstream service while Git switches branches.
	// Scan every local branch tip so commits on the departed branch survive,
	// carrying that branch's ticket context rather than the eventual HEAD's.
	refs, err := gm.repo.References()
	if err != nil {
		log.Printf("Read branch refs: %v", err)
		return
	}
	var tips []*plumbing.Reference
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() {
			tips = append(tips, ref)
		}
		return nil
	})
	if err != nil {
		log.Printf("Read branch refs: %v", err)
		return
	}
	def := gm.DefaultBranch()
	sort.Slice(tips, func(i, j int) bool {
		// Visit source branches before the default branch's merge ancestry.
		if tips[i].Name().Short() == def {
			return false
		}
		if tips[j].Name().Short() == def {
			return true
		}
		return tips[i].Name() < tips[j].Name()
	})
	if !head.Name().IsBranch() {
		tips = append(tips, head)
	}
	for _, ref := range tips {
		gm.checkTip(ref, onCommit)
	}
}

func (gm *GitMonitor) checkTip(ref *plumbing.Reference, onCommit func(CommitInfo)) {
	branch := ""
	if ref.Name().IsBranch() {
		branch = ref.Name().Short()
	}
	var commits []*object.Commit
	visiting := make(map[plumbing.Hash]bool)
	var walk func(plumbing.Hash) error
	walk = func(hash plumbing.Hash) error {
		if gm.seen[hash] || visiting[hash] {
			return nil
		}
		visiting[hash] = true
		commit, err := gm.repo.CommitObject(hash)
		if err != nil {
			return err
		}
		for _, parent := range commit.ParentHashes {
			if err := walk(parent); err != nil {
				return err
			}
		}
		commits = append(commits, commit)
		return nil
	}
	if err := walk(ref.Hash()); err != nil {
		log.Printf("Read new commits: %v", err)
		return
	}
	for _, commit := range commits {
		files, err := gm.getChangedFiles(commit)
		if err != nil {
			log.Printf("Read changed files: %v", err)
		}
		onCommit(CommitInfo{Hash: commit.Hash.String(), Message: strings.TrimSpace(commit.Message),
			Author: commit.Author.Name, Timestamp: commit.Author.When, Files: files,
			Branch: branch, IsMerge: commit.NumParents() >= 2})
		gm.seen[commit.Hash] = true
	}
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

	branch := ""
	if ref.Name().IsBranch() {
		branch = ref.Name().Short()
	}

	return &CommitInfo{
		Hash:      commit.Hash.String(),
		Message:   strings.TrimSpace(commit.Message),
		Author:    commit.Author.Name,
		Timestamp: commit.Author.When,
		Files:     files,
		Branch:    branch,
		IsMerge:   commit.NumParents() >= 2,
	}, nil
}

// DefaultBranch returns the repository's default branch name. It prefers the
// remote HEAD symbolic ref (refs/remotes/origin/HEAD → origin's default), and
// falls back to a local "main" or "master" branch. Returns "" when neither
// can be determined (e.g. a repo with no remote and a custom trunk name).
func (gm *GitMonitor) DefaultBranch() string {
	if ref, err := gm.repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), false); err == nil &&
		ref.Type() == plumbing.SymbolicReference {
		// Target is e.g. refs/remotes/origin/main → default branch "main".
		target := ref.Target().String()
		if name, ok := strings.CutPrefix(target, "refs/remotes/origin/"); ok && name != "" {
			return name
		}
	}
	for _, name := range []string{"main", "master"} {
		if _, err := gm.repo.Reference(plumbing.NewBranchReferenceName(name), false); err == nil {
			return name
		}
	}
	return ""
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

	// Resolve the commit-log path, colocated with daemon.log under the data home.
	// Older builds wrote it relative to DEVTRACK_HOME, which could be empty and
	// produced a broken hook; pin it to the log dir (with a data-home fallback).
	logDir := config.GetLogDir()
	if logDir == "" {
		if dataHome, err := config.DevtrackDataHome(); err == nil {
			logDir = filepath.Join(dataHome, "data", "logs")
		}
	}

	// Git hooks run inside Git Bash on Windows, which requires POSIX-style paths.
	// Convert D:\path\to\file → /d/path/to/file so >> redirection works correctly.
	commitLogPath := filepath.Join(logDir, "commit.log")
	if runtime.GOOS == "windows" && len(commitLogPath) >= 2 && commitLogPath[1] == ':' {
		commitLogPath = "/" + strings.ToLower(string(commitLogPath[0])) + "/" + filepath.ToSlash(commitLogPath[3:])
	} else {
		commitLogPath = filepath.ToSlash(commitLogPath)
	}
	hookContent := postCommitHookContent(commitLogPath)

	// If a hook already exists, only replace it when it is a stale devtrack-managed
	// hook. Never clobber a user's own post-commit hook.
	if existing, err := os.ReadFile(hookPath); err == nil {
		if !strings.Contains(string(existing), "Auto-generated by devtrack") {
			log.Printf("Post-commit hook exists and is not devtrack-managed; leaving it untouched: %s", hookPath)
			return nil
		}
		if string(existing) == hookContent {
			return nil // already up to date
		}
		log.Printf("Upgrading stale devtrack post-commit hook at: %s", hookPath)
	}

	// Write the hook file
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("failed to create post-commit hook: %w", err)
	}

	log.Printf("✓ Installed post-commit hook at: %s", hookPath)
	return nil
}

func postCommitHookContent(commitLogPath string) string {
	return fmt.Sprintf(`#!/bin/sh
# Auto-generated by devtrack - Git commit detection hook
# This hook notifies the devtrack daemon about new commits

# Notify the daemon. Keep the hook fail-open and completely silent even when
# the log destination is temporarily unavailable.
{ echo "Commit detected at $(date)" >> "%s"; } 2>/dev/null || true

exit 0
`, commitLogPath)
}

// InstallPrePushHook installs a pre-push hook that drains the deferred-commit
// queue, retrying AI enhancement right before a push ("when a push was supposed
// to be done"). It always exits 0 so it can never block a push, and skips when
// DEVTRACK_NO_PREPUSH is set. It never clobbers a user's own pre-push hook.
func InstallPrePushHook(repoPath string) error {
	hookPath := filepath.Join(repoPath, ".git", "hooks", "pre-push")

	hookContent := `#!/bin/sh
# Auto-generated by devtrack - deferred-commit enhancement on push
# Retries AI enhancement of any commits queued while the LLM was offline.
# Never blocks the push: always exits 0.

if [ -z "$DEVTRACK_NO_PREPUSH" ] && command -v devtrack >/dev/null 2>&1; then
    devtrack commits enhance >/dev/null 2>&1 || true
fi

exit 0
`

	// Only replace a stale devtrack-managed hook; leave a user's hook untouched.
	if existing, err := os.ReadFile(hookPath); err == nil {
		if !strings.Contains(string(existing), "Auto-generated by devtrack") {
			log.Printf("Pre-push hook exists and is not devtrack-managed; leaving it untouched: %s", hookPath)
			return nil
		}
		if string(existing) == hookContent {
			return nil // already up to date
		}
		log.Printf("Upgrading stale devtrack pre-push hook at: %s", hookPath)
	}

	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("failed to create pre-push hook: %w", err)
	}

	log.Printf("✓ Installed pre-push hook at: %s", hookPath)
	return nil
}

// IsGitRepository checks if a directory is a Git repository
func IsGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}
