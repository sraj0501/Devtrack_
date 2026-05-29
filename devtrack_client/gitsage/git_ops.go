package gitsage

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GitOps provides structured access to git operations in a repository.
type GitOps struct {
	RepoPath string
}

// NewGitOps creates a GitOps for the given repository path.
func NewGitOps(repoPath string) *GitOps {
	return &GitOps{RepoPath: repoPath}
}

// run executes a git command and returns trimmed stdout+stderr.
func (g *GitOps) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoPath
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// runLines executes a git command and returns each non-empty output line.
func (g *GitOps) runLines(args ...string) ([]string, error) {
	out, err := g.run(args...)
	if out == "" {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, err
}

// --- Status & diff ---

// StatusEntry represents a single file in git status.
type StatusEntry struct {
	XY   string // two-char status code (e.g. "M ", " M", "??", "UU")
	Path string
}

// Status returns parsed git status entries.
func (g *GitOps) Status() ([]StatusEntry, error) {
	lines, err := g.runLines("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	entries := make([]StatusEntry, 0, len(lines))
	for _, l := range lines {
		if len(l) < 4 {
			continue
		}
		entries = append(entries, StatusEntry{
			XY:   l[:2],
			Path: strings.TrimSpace(l[3:]),
		})
	}
	return entries, nil
}

// StagedFiles returns a list of files currently staged (index modified).
func (g *GitOps) StagedFiles() ([]string, error) {
	lines, err := g.runLines("diff", "--cached", "--name-only")
	return lines, err
}

// UnstagedFiles returns a list of modified but unstaged files.
func (g *GitOps) UnstagedFiles() ([]string, error) {
	lines, err := g.runLines("diff", "--name-only")
	return lines, err
}

// UntrackedFiles returns a list of untracked files.
func (g *GitOps) UntrackedFiles() ([]string, error) {
	lines, err := g.runLines("ls-files", "--others", "--exclude-standard")
	return lines, err
}

// DiffCached returns the full diff of staged changes.
func (g *GitOps) DiffCached() (string, error) {
	return g.run("diff", "--cached")
}

// DiffFull returns the full diff of unstaged changes.
func (g *GitOps) DiffFull() (string, error) {
	return g.run("diff")
}

// --- Log ---

// LogEntry represents a single git log entry.
type LogEntry struct {
	Hash    string
	Author  string
	Date    string
	Subject string
}

// Log returns the last N commits as structured entries.
func (g *GitOps) Log(n int) ([]LogEntry, error) {
	sep := "\x00"
	format := fmt.Sprintf("--format=%%H%s%%an%s%%ai%s%%s", sep, sep, sep)
	lines, err := g.runLines("log", format, fmt.Sprintf("-n%d", n))
	if err != nil {
		return nil, err
	}
	var entries []LogEntry
	for _, l := range lines {
		parts := strings.SplitN(l, sep, 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, LogEntry{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Subject: parts[3],
		})
	}
	return entries, nil
}

// HEAD returns the current HEAD commit hash.
func (g *GitOps) HEAD() (string, error) {
	return g.run("rev-parse", "HEAD")
}

// --- Branch operations ---

// CurrentBranch returns the current branch name.
func (g *GitOps) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

// Branches returns a list of local branch names.
func (g *GitOps) Branches() ([]string, error) {
	lines, err := g.runLines("branch", "--format=%(refname:short)")
	return lines, err
}

// CreateBranch creates and checks out a new branch.
func (g *GitOps) CreateBranch(name string) (string, error) {
	return g.run("checkout", "-b", name)
}

// CheckoutBranch checks out an existing branch.
func (g *GitOps) CheckoutBranch(name string) (string, error) {
	return g.run("checkout", name)
}

// --- Staging & commits ---

// Add stages one or more paths. Use "." to stage all.
func (g *GitOps) Add(paths ...string) (string, error) {
	args := append([]string{"add"}, paths...)
	return g.run(args...)
}

// Commit creates a new commit with the given message.
func (g *GitOps) Commit(message string) (string, error) {
	return g.run("commit", "-m", message)
}

// CommitAmend amends the previous commit (message only, keeps staged changes).
func (g *GitOps) CommitAmend(message string) (string, error) {
	return g.run("commit", "--amend", "-m", message)
}

// ResetSoft moves HEAD back N commits, keeping changes staged.
func (g *GitOps) ResetSoft(n int) (string, error) {
	return g.run("reset", "--soft", fmt.Sprintf("HEAD~%d", n))
}

// ResetMixed moves HEAD back N commits, unstaging changes.
func (g *GitOps) ResetMixed(n int) (string, error) {
	return g.run("reset", "HEAD~"+strconv.Itoa(n))
}

// ResetToRef resets to a specific commit hash (mixed — keeps working tree changes).
func (g *GitOps) ResetToRef(ref string) (string, error) {
	return g.run("reset", ref)
}

// --- Stash ---

// Stash saves the current working directory state to the stash.
func (g *GitOps) Stash(message string) (string, error) {
	if message != "" {
		return g.run("stash", "push", "-m", message)
	}
	return g.run("stash", "push")
}

// StashPop applies the most recent stash and removes it.
func (g *GitOps) StashPop() (string, error) {
	return g.run("stash", "pop")
}

// StashList returns stash entries.
func (g *GitOps) StashList() ([]string, error) {
	return g.runLines("stash", "list")
}

// --- Merge & remote ---

// Merge merges the given branch into the current branch.
func (g *GitOps) Merge(branch string) (string, error) {
	return g.run("merge", branch)
}

// MergeAbort aborts an in-progress merge.
func (g *GitOps) MergeAbort() (string, error) {
	return g.run("merge", "--abort")
}

// Pull fetches and merges from the default remote.
func (g *GitOps) Pull() (string, error) {
	return g.run("pull")
}

// Push pushes the current branch to the default remote.
func (g *GitOps) Push() (string, error) {
	return g.run("push")
}

// PushBranch pushes the current branch to the named remote.
func (g *GitOps) PushBranch(remote, branch string) (string, error) {
	return g.run("push", remote, branch)
}

// --- Blame ---

// BlameLine returns the commit info for a specific line in a file.
func (g *GitOps) BlameLine(file string, line int) (string, error) {
	return g.run("blame", "-L", fmt.Sprintf("%d,%d", line, line), "--porcelain", "--", file)
}

// --- Utility ---

// IsRepo returns true if the path is a valid git repository.
func (g *GitOps) IsRepo() bool {
	_, err := g.run("rev-parse", "--git-dir")
	return err == nil
}

// RemoteURL returns the URL for the given remote (default: origin).
func (g *GitOps) RemoteURL(remote string) (string, error) {
	return g.run("remote", "get-url", remote)
}
