package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// postJSON executes an authenticated POST against the /api/v4 base with a JSON
// body and decodes the response into out (out may be nil).
func (c *Client) postJSON(path string, body any, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v4"+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("gitlab API error %d: %s", resp.StatusCode, string(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("gitlab JSON decode: %w", err)
		}
	}
	return nil
}

// AddIssueNote posts a note (comment) to an issue. projectPath is "group/project"
// (URL-encoded automatically); iid is the issue's project-scoped IID.
func (c *Client) AddIssueNote(projectPath string, iid int, body string) error {
	path := fmt.Sprintf("/projects/%s/issues/%d/notes", url.QueryEscape(projectPath), iid)
	return c.postJSON(path, map[string]string{"body": body}, nil)
}
