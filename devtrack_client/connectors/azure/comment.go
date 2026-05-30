package azure

import "fmt"

// commentsAPIVersion is the work-item comments API version (preview).
const commentsAPIVersion = "7.1-preview.3"

// AddWorkItemComment posts a comment to a work item using the comments API.
func (c *Client) AddWorkItemComment(id int, text string) error {
	url := fmt.Sprintf("%s/_apis/wit/workItems/%d/comments?api-version=%s",
		c.projectURL(), id, commentsAPIVersion)
	return c.post(url, map[string]string{"text": text}, nil)
}
