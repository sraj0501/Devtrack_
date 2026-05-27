package github

import (
	"fmt"
	"strings"
)

// ViewIssue fetches a single GitHub issue by owner, repo, and number.
func ViewIssue(owner, repo string, number int) (*Issue, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}

	var issue Issue
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number)
	if err := c.do(path, &issue); err != nil {
		return nil, err
	}
	issue.Repo = owner + "/" + repo
	return &issue, nil
}

// FormatIssue returns a human-readable multi-line summary of an issue.
func FormatIssue(iss *Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "GitHub Issue #%d — %s\n", iss.Number, iss.Title)
	fmt.Fprintf(&b, "State:      %s\n", iss.State)
	fmt.Fprintf(&b, "Repo:       %s\n", iss.Repo)
	fmt.Fprintf(&b, "URL:        %s\n", iss.URL)
	fmt.Fprintf(&b, "Updated:    %s\n", iss.UpdatedAt)
	if labels := iss.LabelNames(); labels != "" {
		fmt.Fprintf(&b, "Labels:     %s\n", labels)
	}
	if iss.Body != "" {
		fmt.Fprintf(&b, "\n%s\n", truncate(iss.Body, 500))
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
