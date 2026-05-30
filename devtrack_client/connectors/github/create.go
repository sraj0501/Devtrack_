package github

import "fmt"

// createdIssue captures the fields we need from a created-issue response.
type createdIssue struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
}

// CreateIssue opens a new issue in repo ("owner/repo") and returns its number
// and HTML URL. milestone is the milestone number, or 0 for none.
func CreateIssue(repo, title, body string, milestone int) (int, string, error) {
	c, err := NewClient()
	if err != nil {
		return 0, "", err
	}
	payload := map[string]any{"title": title}
	if body != "" {
		payload["body"] = body
	}
	if milestone > 0 {
		payload["milestone"] = milestone
	}
	var out createdIssue
	if err := c.postJSON(fmt.Sprintf("/repos/%s/issues", repo), payload, &out); err != nil {
		return 0, "", err
	}
	return out.Number, out.URL, nil
}
