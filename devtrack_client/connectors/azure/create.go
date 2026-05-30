package azure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// patchOp is a single JSON-Patch operation used to create/update work items.
type patchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// createdWorkItem captures the fields we need from a created-work-item response.
type createdWorkItem struct {
	ID    int            `json:"id"`
	Links map[string]any `json:"_links"`
}

// CreateWorkItem creates a work item of workItemType (defaults to "Task") and
// returns its id and web URL. The work-item create endpoint requires a
// JSON-Patch document and the application/json-patch+json content type, so this
// builds the request directly rather than reusing post().
func CreateWorkItem(title, description, workItemType string) (int, string, error) {
	c, err := NewClient()
	if err != nil {
		return 0, "", err
	}
	if strings.TrimSpace(workItemType) == "" {
		workItemType = "Task"
	}

	ops := []patchOp{{Op: "add", Path: "/fields/System.Title", Value: title}}
	if description != "" {
		ops = append(ops, patchOp{Op: "add", Path: "/fields/System.Description", Value: description})
	}

	safeType := strings.ReplaceAll(workItemType, " ", "%20")
	url := fmt.Sprintf("%s/_apis/wit/workitems/$%s?api-version=%s",
		c.projectURL(), safeType, apiVersion)

	encoded, err := json.Marshal(ops)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json-patch+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("azure request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", err
	}
	if resp.StatusCode >= 400 {
		return 0, "", fmt.Errorf("azure API error %d: %s", resp.StatusCode, string(respBody))
	}

	var out createdWorkItem
	if err := json.Unmarshal(respBody, &out); err != nil {
		return 0, "", fmt.Errorf("azure JSON decode: %w", err)
	}
	return out.ID, workItemWebURL(out.Links), nil
}

// workItemWebURL pulls the human-facing URL from a work item's _links.html.href.
func workItemWebURL(links map[string]any) string {
	if links == nil {
		return ""
	}
	if h, ok := links["html"].(map[string]any); ok {
		if href, ok := h["href"].(string); ok {
			return href
		}
	}
	return ""
}
