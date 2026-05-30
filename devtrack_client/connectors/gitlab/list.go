package gitlab

import "fmt"

// ListIssues returns all open issues assigned to username.
// username comes from workspaces.yaml pm_username; pass "" to auto-detect
// from the /user API.
func (c *Client) ListIssues(username string) ([]Issue, error) {
	if username == "" {
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
func extractRepoFromURL(webURL string) string {
	idx := indexOf(webURL, "/-/issues")
	if idx < 0 {
		return webURL
	}
	path := webURL[:idx]
	for _, prefix := range []string{"https://gitlab.com/", "http://gitlab.com/"} {
		if len(path) > len(prefix) && path[:len(prefix)] == prefix {
			return path[len(prefix):]
		}
	}
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
