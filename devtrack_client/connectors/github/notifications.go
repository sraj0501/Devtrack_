package github

import "time"

// GitHubNotification is a single entry from the GitHub Notifications API.
type GitHubNotification struct {
	ID     string `json:"id"`
	Reason string `json:"reason"` // assigned, comment, review_requested, state_change, mention, …
	Unread bool   `json:"unread"`
	Subject struct {
		Title            string `json:"title"`
		URL              string `json:"url"`              // API URL for the issue/PR
		LatestCommentURL string `json:"latest_comment_url"`
		Type             string `json:"type"` // Issue, PullRequest, Commit
	} `json:"subject"`
	Repository struct {
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	UpdatedAt string `json:"updated_at"`
}

// ListNotificationsSince returns unread notifications updated after since.
// Pass zero time to get all unread notifications (GitHub defaults to last 30 days).
func (c *Client) ListNotificationsSince(since time.Time) ([]GitHubNotification, error) {
	path := "/notifications?all=false"
	if !since.IsZero() {
		path += "&since=" + since.UTC().Format(time.RFC3339)
	}
	var out []GitHubNotification
	if err := c.do(path, &out); err != nil {
		return nil, err
	}
	return out, nil
}
