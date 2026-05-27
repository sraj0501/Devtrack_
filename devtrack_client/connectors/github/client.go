package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// Client is a minimal GitHub REST API client.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient creates a GitHub client using GITHUB_TOKEN from the environment.
// GITHUB_API_URL overrides the base URL (useful for GitHub Enterprise).
func NewClient() (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is not set")
	}
	base := os.Getenv("GITHUB_API_URL")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		token:   token,
		baseURL: strings.TrimRight(base, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// do executes an authenticated GET request and decodes the JSON body into out.
func (c *Client) do(path string, out any) error {
	url := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("github API error %d: %s", resp.StatusCode, string(body))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("github JSON decode: %w", err)
		}
	}
	return nil
}

// Issue represents a GitHub issue returned by the API.
type Issue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	URL       string   `json:"html_url"`
	State     string   `json:"state"`
	UpdatedAt string   `json:"updated_at"`
	Labels    []label  `json:"labels"`
	Repo      string   // populated by caller from context
}

type label struct {
	Name string `json:"name"`
}

// LabelNames returns a comma-separated list of label names.
func (i Issue) LabelNames() string {
	names := make([]string, len(i.Labels))
	for idx, l := range i.Labels {
		names[idx] = l.Name
	}
	return strings.Join(names, ",")
}

// AuthenticatedUser holds minimal user info from /user.
type AuthenticatedUser struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	RateLimit struct {
		Limit     int `json:"limit"`
		Remaining int `json:"remaining"`
	}
}
