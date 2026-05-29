package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// postJSON executes an authenticated POST with a JSON body and decodes the
// response into out (out may be nil).
func (c *Client) postJSON(path string, body any, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("github API error %d: %s", resp.StatusCode, string(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("github JSON decode: %w", err)
		}
	}
	return nil
}

// AddIssueComment posts a comment to an issue. repo is "owner/repo".
func AddIssueComment(repo string, number int, body string) error {
	c, err := NewClient()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number)
	return c.postJSON(path, map[string]string{"body": body}, nil)
}
