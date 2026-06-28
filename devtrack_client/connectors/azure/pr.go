package azure

import "fmt"

// PRReviewer represents a single reviewer entry on an Azure DevOps pull request.
// Vote values: 10=Approved, 5=Approved with suggestions, 0=No vote,
// -5=Waiting for author, -10=Rejected.
type PRReviewer struct {
	DisplayName string `json:"displayName"`
	Vote        int    `json:"vote"`
}

// prSearchItem is the per-PR element returned by the pull-requests search endpoint.
type prSearchItem struct {
	PullRequestID int          `json:"pullRequestId"`
	Reviewers     []PRReviewer `json:"reviewers"`
}

// prSearchResponse is the top-level envelope from
// GET {projectURL}/_apis/git/pullrequests?pullRequestId={id}&api-version=7.0
type prSearchResponse struct {
	Value []prSearchItem `json:"value"`
	Count int            `json:"count"`
}

// ListPRReviewers returns all reviewers (with their votes) for the pull request
// identified by prID, searching across all repositories in the project.
// It returns an error if the PR is not found.
func (c *Client) ListPRReviewers(prID int) ([]PRReviewer, error) {
	url := fmt.Sprintf("%s/_apis/git/pullrequests?pullRequestId=%d&api-version=%s",
		c.projectURL(), prID, apiVersion)

	var resp prSearchResponse
	if err := c.get(url, &resp); err != nil {
		return nil, fmt.Errorf("azure ListPRReviewers(prID=%d): %w", prID, err)
	}

	if resp.Count == 0 || len(resp.Value) == 0 {
		return nil, fmt.Errorf("azure ListPRReviewers(prID=%d): PR not found", prID)
	}

	return resp.Value[0].Reviewers, nil
}
