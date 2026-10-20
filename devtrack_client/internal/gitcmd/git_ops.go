package gitcmd

import (
	"bytes"
	"os/exec"
	"strings"
)

// GitOps provides the small read-only Git surface used by the explicit
// `devtrack git` commit workflow and its ticket-linking hooks.
type GitOps struct {
	RepoPath string
}

func NewGitOps(repoPath string) *GitOps {
	return &GitOps{RepoPath: repoPath}
}

func (g *GitOps) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoPath
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func (g *GitOps) runLines(args ...string) ([]string, error) {
	out, err := g.run(args...)
	if out == "" {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, err
}

func (g *GitOps) StagedFiles() ([]string, error) {
	return g.runLines("diff", "--cached", "--name-only")
}

func (g *GitOps) DiffCached() (string, error) {
	return g.run("diff", "--cached")
}

func (g *GitOps) DiffFull() (string, error) {
	return g.run("diff")
}

func (g *GitOps) HEAD() (string, error) {
	return g.run("rev-parse", "HEAD")
}

func (g *GitOps) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

func (g *GitOps) IsRepo() bool {
	_, err := g.run("rev-parse", "--git-dir")
	return err == nil
}
