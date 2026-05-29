package gitlab

import (
	"fmt"
	"os"
)

// ListIssues returns all open issues assigned to the authenticated user.
func ListIssues() ([]Issue, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}

	username := os.Getenv("GITLAB_USERNAME")
	if username == "" {
		// Fall back to fetching from API
		var user AuthenticatedUser
		if err := c.do("/user", &user); err != nil {
			return nil, fmt.Errorf("gitlab list: cannot determine username: %w", err)
		}
		username = user.Username
	}

	var all []Issue
	page := 1
	for {
		var batch []Issue
		path := fmt.Sprintf("/issues?assignee_username=%s&state=opened&per_page=50&page=%d",
			username, page)
		if err := c.do(path, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for i := range batch {
			batch[i].Repo = extractRepoFromURL(batch[i].URL)
		}
		all = append(all, batch...)
		if len(batch) < 50 {
			break
		}
		page++
	}
	return all, nil
}

// extractRepoFromURL parses "group/project" from a GitLab issue web URL.
// e.g. https://gitlab.com/group/project/-/issues/42 → group/project
func extractRepoFromURL(webURL string) string {
	// Find /-/issues part and strip it
	idx := indexOf(webURL, "/-/issues")
	if idx < 0 {
		return webURL
	}
	// Strip the base URL up to the first /
	path := webURL[:idx]
	// Find last occurrence of https://gitlab.com/ or https://instance/
	for _, prefix := range []string{"https://gitlab.com/", "http://gitlab.com/"} {
		if len(path) > len(prefix) && path[:len(prefix)] == prefix {
			return path[len(prefix):]
		}
	}
	// Self-hosted: strip up to third slash
	count := 0
	for i, ch := range path {
		if ch == '/' {
			count++
			if count == 3 {
				return path[i+1:]
			}
		}
	}
	return path
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
