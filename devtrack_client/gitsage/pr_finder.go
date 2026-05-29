package gitsage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PRInfo holds metadata about a pull/merge request inferred from the local repo.
type PRInfo struct {
	Number     int    // PR/MR number if detectable
	Branch     string // current branch
	BaseBranch string // likely base branch (main/master/dev)
	Title      string // inferred from branch name or recent commits
	DiffStat   string // lines changed vs base branch
	CommitLog  string // commits ahead of base
}

var (
	// prNumberPatterns matches common branch naming conventions that embed a PR/issue number.
	prNumberPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:^|[-/])(?:pr|gh|issue|fix|feat|feature|chore|bug|task|TASK|PROJ|MR)[-#]?(\d+)`),
		regexp.MustCompile(`[/-](\d{2,6})(?:[/-]|$)`), // bare number in branch path
	}

	// baseBranchCandidates are checked in order; first one that exists wins.
	baseBranchCandidates = []string{"main", "master", "dev", "develop", "trunk"}
)

// FindPR inspects the local repository and returns a PRInfo.
// It does not make any network calls — all data comes from git.
func FindPR(repoPath string) (*PRInfo, error) {
	g := NewGitOps(repoPath)

	branch, err := g.CurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("pr_finder: cannot get current branch: %w", err)
	}

	info := &PRInfo{Branch: branch}
	info.Number = extractPRNumber(branch)
	info.BaseBranch = detectBaseBranch(g)
	info.Title = inferTitle(branch)

	if info.BaseBranch != "" {
		info.DiffStat = diffStat(g, info.BaseBranch)
		info.CommitLog = commitsAhead(g, info.BaseBranch)
	}

	return info, nil
}

// extractPRNumber attempts to parse a PR/issue number from the branch name.
// Returns 0 if none found.
func extractPRNumber(branch string) int {
	for _, re := range prNumberPatterns {
		if m := re.FindStringSubmatch(branch); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

// detectBaseBranch returns the first of the canonical base branches that exists locally.
func detectBaseBranch(g *GitOps) string {
	branches, err := g.Branches()
	if err != nil {
		return ""
	}
	set := make(map[string]bool, len(branches))
	for _, b := range branches {
		set[strings.TrimSpace(b)] = true
	}
	for _, candidate := range baseBranchCandidates {
		if set[candidate] {
			return candidate
		}
	}
	return ""
}

// inferTitle converts a branch name into a human-readable title.
// e.g. "feature/go-client-standalone" → "Go client standalone"
func inferTitle(branch string) string {
	// Strip common prefixes
	for _, prefix := range []string{"feature/", "feat/", "fix/", "bugfix/", "chore/", "hotfix/", "refactor/"} {
		branch = strings.TrimPrefix(branch, prefix)
	}
	// Replace separators with spaces
	branch = strings.NewReplacer("-", " ", "_", " ", "/", " ").Replace(branch)
	// Strip leading PR number tokens
	branch = regexp.MustCompile(`^\d+\s+`).ReplaceAllString(branch, "")
	if branch == "" {
		return ""
	}
	return strings.ToUpper(branch[:1]) + branch[1:]
}

// diffStat returns a short summary of lines changed between the current branch and base.
func diffStat(g *GitOps, base string) string {
	out, err := g.run("diff", "--stat", base+"...HEAD")
	if err != nil || out == "" {
		return ""
	}
	// Return only the summary line (last line of diff --stat)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// commitsAhead returns the one-line log of commits on the current branch not in base.
func commitsAhead(g *GitOps, base string) string {
	out, err := g.run("log", "--oneline", base+"...HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Format returns a human-readable summary of the PR info for LLM context injection.
func (p *PRInfo) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Branch: %s", p.Branch)
	if p.Number > 0 {
		fmt.Fprintf(&b, " (PR/Issue #%d)", p.Number)
	}
	fmt.Fprintln(&b)
	if p.Title != "" {
		fmt.Fprintf(&b, "Title: %s\n", p.Title)
	}
	if p.BaseBranch != "" {
		fmt.Fprintf(&b, "Base branch: %s\n", p.BaseBranch)
	}
	if p.DiffStat != "" {
		fmt.Fprintf(&b, "Changes vs %s: %s\n", p.BaseBranch, p.DiffStat)
	}
	if p.CommitLog != "" {
		fmt.Fprintf(&b, "Commits ahead:\n%s\n", p.CommitLog)
	}
	return b.String()
}
