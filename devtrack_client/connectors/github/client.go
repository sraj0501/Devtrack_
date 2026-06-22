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
// Construct with NewClient — token is read from GITHUB_TOKEN in .env;
// all other config (apiURL) comes from workspaces.yaml via the caller.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient creates a GitHub client.
// token is read from GITHUB_TOKEN (.env — the only env read this package does).
// apiURL overrides the base URL (GitHub Enterprise); pass "" for github.com.
func NewClient(apiURL string) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is not set")
	}
	base := apiURL
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
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	URL       string  `json:"html_url"`
	State     string  `json:"state"`
	UpdatedAt string  `json:"updated_at"`
	Labels    []label `json:"labels"`
	Repo      string  // populated by caller from context
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
	Login string `json:"login"`
	Name  string `json:"name"`
}

// PullRequest is a minimal PR representation from the GitHub REST API.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	HTMLURL string `json:"html_url"`
	Head    struct {
		Ref string `json:"ref"` // branch name
	} `json:"head"`
}

// PRReviewComment is a single review comment on a pull request.
type PRReviewComment struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// Path is the file the comment is on (for inline comments); empty for top-level.
	Path string `json:"path"`
}

// GetAuthenticatedUser fetches the authenticated user's login.
func (c *Client) GetAuthenticatedUser() (*AuthenticatedUser, error) {
	var u AuthenticatedUser
	if err := c.do("/user", &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListOpenPRsAuthored returns all open PRs in the given repo authored by login.
// repo is "owner/repo".
func (c *Client) ListOpenPRsAuthored(repo, login string) ([]PullRequest, error) {
	var all []PullRequest
	page := 1
	for {
		var batch []PullRequest
		path := fmt.Sprintf("/repos/%s/pulls?state=open&per_page=50&page=%d", repo, page)
		if err := c.do(path, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, pr := range batch {
			if pr.User.Login == login {
				all = append(all, pr)
			}
		}
		if len(batch) < 50 {
			break
		}
		page++
	}
	return all, nil
}

// ListPRReviewComments returns inline review comments for a pull request.
// repo is "owner/repo", prNumber is the PR number.
func (c *Client) ListPRReviewComments(repo string, prNumber int) ([]PRReviewComment, error) {
	var all []PRReviewComment
	page := 1
	for {
		var batch []PRReviewComment
		path := fmt.Sprintf("/repos/%s/pulls/%d/comments?per_page=100&page=%d", repo, prNumber, page)
		if err := c.do(path, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
		page++
	}
	return all, nil
}

// ListPRIssueComments returns top-level (non-inline) comments on a PR.
// These are returned by the issues comments API.
func (c *Client) ListPRIssueComments(repo string, prNumber int) ([]PRReviewComment, error) {
	var all []PRReviewComment
	page := 1
	for {
		var batch []struct {
			ID      int64  `json:"id"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}
		path := fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100&page=%d", repo, prNumber, page)
		if err := c.do(path, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, b := range batch {
			all = append(all, PRReviewComment{
				ID:        b.ID,
				Body:      b.Body,
				HTMLURL:   b.HTMLURL,
				User:      struct{ Login string `json:"login"` }{Login: b.User.Login},
				CreatedAt: b.CreatedAt,
				UpdatedAt: b.UpdatedAt,
			})
		}
		if len(batch) < 100 {
			break
		}
		page++
	}
	return all, nil
}
