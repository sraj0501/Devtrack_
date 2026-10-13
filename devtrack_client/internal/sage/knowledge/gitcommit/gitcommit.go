// Package gitcommit creates local commits containing only Sage-owned knowledge
// writes. It never pushes and never mutates the user's real Git index.
package gitcommit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Change struct {
	Path          string
	Before, After []byte
}

// Recorder coalesces repeated atomic writes while retaining the first baseline.
type Recorder struct {
	mu      sync.Mutex
	changes map[string]Change
}

func (r *Recorder) Record(path string, before, after []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.changes == nil {
		r.changes = make(map[string]Change)
	}
	change, ok := r.changes[path]
	if !ok {
		change = Change{Path: path, Before: clone(before)}
	}
	change.After = clone(after)
	r.changes[path] = change
}

func (r *Recorder) Changes() []Change {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Change, 0, len(r.changes))
	for _, change := range r.changes {
		out = append(out, Change{Path: change.Path, Before: clone(change.Before), After: clone(change.After)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func clone(raw []byte) []byte {
	if raw == nil {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

type Committer struct {
	KnowledgeRoot string
	Enabled       bool
	GitExecutable string
	Message       string
}

func (c Committer) git() string {
	if strings.TrimSpace(c.GitExecutable) != "" {
		return c.GitExecutable
	}
	return "git"
}

func (c Committer) run(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, c.git(), args...)
	command.Dir = dir
	command.Env = append(os.Environ(), env...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("git %s: %w", args[0], err)
	}
	return output, nil
}

func (c Committer) Commit(ctx context.Context, changes []Change) (bool, error) {
	if !c.Enabled || len(changes) == 0 {
		return false, nil
	}
	root, err := filepath.Abs(c.KnowledgeRoot)
	if err != nil {
		return false, err
	}
	repoRaw, err := c.run(ctx, root, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		// A knowledge directory outside Git is a supported no-op.
		return false, nil
	}
	repo := strings.TrimSpace(string(repoRaw))
	rootRel, err := filepath.Rel(repo, root)
	if err != nil || escapes(rootRel) {
		return false, nil
	}

	var paths []string
	for _, change := range changes {
		path, err := filepath.Abs(change.Path)
		if err != nil {
			continue
		}
		relRoot, err := filepath.Rel(root, path)
		if err != nil || escapes(relRoot) || !owned(relRoot) {
			continue
		}
		relRepo, err := filepath.Rel(repo, path)
		if err != nil || escapes(relRepo) {
			continue
		}
		baseline, exists, err := c.headFile(ctx, repo, filepath.ToSlash(relRepo))
		if err != nil || exists != (change.Before != nil) || !bytes.Equal(baseline, change.Before) {
			// The file had user work before Sage touched it. Never guess ownership.
			continue
		}
		current, readErr := os.ReadFile(path)
		if change.After == nil {
			if !errors.Is(readErr, os.ErrNotExist) {
				continue
			}
		} else if readErr != nil || !bytes.Equal(current, change.After) {
			// A later edit raced with the writer; leave it entirely alone.
			continue
		}
		paths = append(paths, filepath.ToSlash(relRepo))
	}
	if len(paths) == 0 {
		return false, nil
	}
	sort.Strings(paths)
	index, err := os.CreateTemp("", "devtrack-sage-index-*")
	if err != nil {
		return false, err
	}
	indexPath := index.Name()
	if err := index.Close(); err != nil {
		return false, err
	}
	if err := os.Remove(indexPath); err != nil {
		return false, err
	}
	defer os.Remove(indexPath)
	env := []string{"GIT_INDEX_FILE=" + indexPath, "GIT_NO_DEVTRACK=1"}
	if _, err := c.run(ctx, repo, env, "read-tree", "HEAD"); err != nil {
		return false, err
	}
	args := append([]string{"add", "-A", "--"}, paths...)
	if _, err := c.run(ctx, repo, env, args...); err != nil {
		return false, err
	}
	command := exec.CommandContext(ctx, c.git(), "diff", "--cached", "--quiet")
	command.Dir, command.Env = repo, append(os.Environ(), env...)
	if err := command.Run(); err == nil {
		return false, nil
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		return false, fmt.Errorf("git diff: %w", err)
	}
	message := strings.TrimSpace(c.Message)
	if message == "" {
		message = "docs(sage): update local knowledge"
	}
	if _, err := c.run(ctx, repo, env, "commit", "--no-verify", "-m", message); err != nil {
		return false, err
	}
	// Advancing HEAD while leaving the real index byte-for-byte untouched would
	// make newly committed Sage paths appear as staged deletions. Refresh only
	// the proven Sage-owned paths; unrelated user staging remains unchanged.
	resetArgs := append([]string{"reset", "-q", "HEAD", "--"}, paths...)
	if _, err := c.run(ctx, repo, nil, resetArgs...); err != nil {
		return true, err
	}
	return true, nil
}

func (c Committer) headFile(ctx context.Context, repo, path string) ([]byte, bool, error) {
	command := exec.CommandContext(ctx, c.git(), "cat-file", "-e", "HEAD:"+path)
	command.Dir = repo
	if err := command.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, false, nil
		}
		return nil, false, err
	}
	raw, err := c.run(ctx, repo, nil, "show", "HEAD:"+path)
	return raw, true, err
}

func escapes(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel)
}

func owned(rel string) bool {
	rel = filepath.ToSlash(rel)
	if strings.Contains(rel, "/") {
		return false
	}
	name := strings.ToLower(rel)
	return name == ".gitignore" || name == "readme.md" || name == "_skipped.md" ||
		(strings.HasSuffix(name, ".md") && !strings.HasPrefix(name, "."))
}
