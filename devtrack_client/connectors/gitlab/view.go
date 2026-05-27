package gitlab

import (
	"fmt"
	"strings"
)

// ViewIssue fetches a single GitLab issue by project ID (URL-encoded path) and IID.
// projectPath is the URL-encoded "group%2Fproject" string, or numeric project ID.
// iid is the issue IID (project-scoped number shown in the UI).
func ViewIssue(projectPath string, iid int) (*Issue, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}

	var issue Issue
	path := fmt.Sprintf("/projects/%s/issues/%d", projectPath, iid)
	if err := c.do(path, &issue); err != nil {
		return nil, err
	}
	issue.Repo = projectPath
	return &issue, nil
}

// FormatIssue returns a human-readable multi-line summary of a GitLab issue.
func FormatIssue(iss *Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "GitLab Issue #%d — %s\n", iss.IID, iss.Title)
	fmt.Fprintf(&b, "State:      %s\n", iss.State)
	fmt.Fprintf(&b, "Repo:       %s\n", iss.Repo)
	fmt.Fprintf(&b, "URL:        %s\n", iss.URL)
	fmt.Fprintf(&b, "Updated:    %s\n", iss.UpdatedAt)
	if labels := iss.LabelNames(); labels != "" {
		fmt.Fprintf(&b, "Labels:     %s\n", labels)
	}
	if iss.Body != "" {
		body := iss.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		fmt.Fprintf(&b, "\n%s\n", body)
	}
	return b.String()
}
