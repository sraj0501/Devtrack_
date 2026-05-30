package gitlab

import (
	"fmt"
	"net/url"
)

// createdIssue captures the fields we need from a created-issue response.
type createdIssue struct {
	IID int    `json:"iid"`
	URL string `json:"web_url"`
}

// CreateIssue opens a new issue in projectPath ("group/project", URL-encoded
// automatically) and returns its project-scoped IID and web URL. milestoneID is
// the milestone id, or 0 for none.
func CreateIssue(projectPath, title, description string, milestoneID int) (int, string, error) {
	c, err := NewClient()
	if err != nil {
		return 0, "", err
	}
	payload := map[string]any{"title": title}
	if description != "" {
		payload["description"] = description
	}
	if milestoneID > 0 {
		payload["milestone_id"] = milestoneID
	}
	path := fmt.Sprintf("/projects/%s/issues", url.QueryEscape(projectPath))
	var out createdIssue
	if err := c.postJSON(path, payload, &out); err != nil {
		return 0, "", err
	}
	return out.IID, out.URL, nil
}
