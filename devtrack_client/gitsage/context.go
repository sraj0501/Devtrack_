package gitsage

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// RepoContext holds a snapshot of the current git repository state.
type RepoContext struct {
	Branch       string
	Status       string
	RecentLog    string
	StagedDiff   string
	UnstagedDiff string
	Remotes      string
}

// CollectContext gathers git state from the current working directory.
func CollectContext(repoPath string) (*RepoContext, error) {
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		_ = cmd.Run()
		return strings.TrimSpace(out.String())
	}

	ctx := &RepoContext{
		Branch:       run("rev-parse", "--abbrev-ref", "HEAD"),
		Status:       run("status", "--short"),
		RecentLog:    run("log", "--oneline", "-10"),
		StagedDiff:   run("diff", "--cached", "--stat"),
		UnstagedDiff: run("diff", "--stat"),
		Remotes:      run("remote", "-v"),
	}
	return ctx, nil
}

// Format returns a human-readable summary of the repo context for LLM injection.
func (c *RepoContext) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Branch: %s\n", c.Branch)
	if c.Status != "" {
		fmt.Fprintf(&b, "Status:\n%s\n", c.Status)
	}
	if c.StagedDiff != "" {
		fmt.Fprintf(&b, "Staged changes:\n%s\n", c.StagedDiff)
	}
	if c.UnstagedDiff != "" {
		fmt.Fprintf(&b, "Unstaged changes:\n%s\n", c.UnstagedDiff)
	}
	if c.RecentLog != "" {
		fmt.Fprintf(&b, "Recent commits:\n%s\n", c.RecentLog)
	}
	return b.String()
}
