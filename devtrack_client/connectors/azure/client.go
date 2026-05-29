package azure

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const apiVersion = "7.0"

// Client is a minimal Azure DevOps REST API client.
type Client struct {
	pat     string
	org     string
	project string
	baseURL string
	http    *http.Client
}

// NewClient creates an Azure DevOps client using env vars:
// AZURE_DEVOPS_PAT, AZURE_ORG, AZURE_PROJECT.
// AZURE_DEVOPS_URL overrides the base URL (useful for Azure DevOps Server).
func NewClient() (*Client, error) {
	pat := os.Getenv("AZURE_DEVOPS_PAT")
	if pat == "" {
		return nil, fmt.Errorf("AZURE_DEVOPS_PAT is not set")
	}
	org := os.Getenv("AZURE_ORG")
	if org == "" {
		return nil, fmt.Errorf("AZURE_ORG is not set")
	}
	project := os.Getenv("AZURE_PROJECT")
	if project == "" {
		return nil, fmt.Errorf("AZURE_PROJECT is not set")
	}

	base := os.Getenv("AZURE_DEVOPS_URL")
	if base == "" {
		base = "https://dev.azure.com"
	}

	return &Client{
		pat:     pat,
		org:     org,
		project: project,
		baseURL: strings.TrimRight(base, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// authHeader returns the Basic auth header value for PAT authentication.
func (c *Client) authHeader() string {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + c.pat))
	return "Basic " + encoded
}

// get executes an authenticated GET request and decodes the JSON body into out.
func (c *Client) get(url string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("azure request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("azure API error %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("azure JSON decode: %w", err)
		}
	}
	return nil
}

// post executes an authenticated POST request with a JSON body and decodes the response.
func (c *Client) post(url string, bodyObj any, out any) error {
	encoded, err := json.Marshal(bodyObj)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("azure request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("azure API error %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("azure JSON decode: %w", err)
		}
	}
	return nil
}

// orgURL returns the base URL for org-level API calls.
func (c *Client) orgURL() string {
	return fmt.Sprintf("%s/%s", c.baseURL, c.org)
}

// projectURL returns the base URL for project-level API calls.
func (c *Client) projectURL() string {
	return fmt.Sprintf("%s/%s/%s", c.baseURL, c.org, c.project)
}

// WorkItem represents an Azure DevOps work item.
type WorkItem struct {
	ID     int                    `json:"id"`
	Fields map[string]any `json:"fields"`
	URL    string                 `json:"url"`
	WebURL string                 // populated from _links if present
}

// Title returns the work item title from fields.
func (w WorkItem) Title() string {
	if v, ok := w.Fields["System.Title"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// State returns the work item state from fields.
func (w WorkItem) State() string {
	if v, ok := w.Fields["System.State"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// AssignedTo returns the assignee display name.
func (w WorkItem) AssignedTo() string {
	if v, ok := w.Fields["System.AssignedTo"]; ok {
		if m, ok := v.(map[string]any); ok {
			if dn, ok := m["displayName"].(string); ok {
				return dn
			}
		}
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// UpdatedAt returns the work item update timestamp.
func (w WorkItem) UpdatedAt() string {
	if v, ok := w.Fields["System.ChangedDate"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// WorkItemType returns the work item type (Bug, Task, User Story, etc).
func (w WorkItem) WorkItemType() string {
	if v, ok := w.Fields["System.WorkItemType"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
