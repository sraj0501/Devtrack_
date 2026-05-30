package github

import "fmt"

// ListIssues returns all open issues assigned to username across all repos.
// username comes from workspaces.yaml pm_username; pass "" to use the
// authenticated user's login (determined via /user API).
func (c *Client) ListIssues(username string) ([]Issue, error) {
	if username == "" {
		var u AuthenticatedUser
		if err := c.do("/user", &u); err != nil {
			return nil, fmt.Errorf("github list: cannot determine username: %w", err)
		}
		username = u.Login
	}

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

// ListIssuesForRepo returns open issues in a specific repo assigned to username.
func (c *Client) ListIssuesForRepo(owner, repo, username string) ([]Issue, error) {
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
func extractRepo(htmlURL string) string {
	prefix := "https://github.com/"
	if len(htmlURL) <= len(prefix) {
		return ""
	}
	rest := htmlURL[len(prefix):]
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
