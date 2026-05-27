package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultGitLabURL = "https://gitlab.com"

// Client is a minimal GitLab REST API v4 client.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient creates a GitLab client using GITLAB_PAT from the environment.
// GITLAB_URL overrides the base URL (useful for self-hosted GitLab).
func NewClient() (*Client, error) {
	token := os.Getenv("GITLAB_PAT")
	if token == "" {
		return nil, fmt.Errorf("GITLAB_PAT is not set")
	}
	base := os.Getenv("GITLAB_URL")
	if base == "" {
		base = defaultGitLabURL
	}
	return &Client{
		token:   token,
		baseURL: strings.TrimRight(base, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// do executes an authenticated GET request and decodes the JSON body into out.
func (c *Client) do(path string, out any) error {
	url := c.baseURL + "/api/v4" + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gitlab API error %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("gitlab JSON decode: %w", err)
		}
	}
	return nil
}

// Issue represents a GitLab issue.
type Issue struct {
	ID        int      `json:"id"`
	IID       int      `json:"iid"`
	Title     string   `json:"title"`
	Body      string   `json:"description"`
	URL       string   `json:"web_url"`
	State     string   `json:"state"`
	UpdatedAt string   `json:"updated_at"`
	Labels    []string `json:"labels"`
	Repo      string   // populated from project_id or web_url
}

// LabelNames returns a comma-joined label string.
func (i Issue) LabelNames() string {
	return strings.Join(i.Labels, ",")
}

// AuthenticatedUser holds minimal user info from /user.
type AuthenticatedUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}
