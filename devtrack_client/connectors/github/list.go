package github

import (
	"fmt"
	"os"
)

// ListIssues returns all open issues assigned to the authenticated user (or username if provided).
// It pages through results until all issues are collected.
func ListIssues(token, username string) ([]Issue, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}
	c.token = token

	var all []Issue
	page := 1
	for {
		var batch []Issue
		path := fmt.Sprintf("/issues?filter=assigned&state=open&per_page=50&page=%d", page)
		if err := c.do(path, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		// Populate repo from the issue URL
		for i := range batch {
			batch[i].Repo = extractRepo(batch[i].URL)
		}
		all = append(all, batch...)
		if len(batch) < 50 {
			break
		}
		page++
	}
	return all, nil
}

// ListIssuesForRepo returns open issues in a specific repo assigned to the user.
func ListIssuesForRepo(owner, repo string) ([]Issue, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}

	username := os.Getenv("GITHUB_USERNAME")
	var all []Issue
	page := 1
	for {
		var batch []Issue
		path := fmt.Sprintf("/repos/%s/%s/issues?state=open&assignee=%s&per_page=50&page=%d",
			owner, repo, username, page)
		if err := c.do(path, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for i := range batch {
			batch[i].Repo = owner + "/" + repo
		}
		all = append(all, batch...)
		if len(batch) < 50 {
			break
		}
		page++
	}
	return all, nil
}

// extractRepo parses "owner/repo" from a GitHub issue HTML URL.
// e.g. https://github.com/owner/repo/issues/42 → owner/repo
func extractRepo(htmlURL string) string {
	// strip https://github.com/
	prefix := "https://github.com/"
	if len(htmlURL) <= len(prefix) {
		return ""
	}
	rest := htmlURL[len(prefix):]
	// rest = owner/repo/issues/42
	parts := splitN(rest, "/", 3)
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return rest
}

func splitN(s, sep string, n int) []string {
	var parts []string
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	parts = append(parts, s)
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
