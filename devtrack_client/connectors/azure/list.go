package azure

import (
	"fmt"
	"strings"
	"time"
)

// wiqlRequest is the body for a WIQL query.
type wiqlRequest struct {
	Query string `json:"query"`
}

// wiqlResponse is the response from the WIQL endpoint.
type wiqlResponse struct {
	WorkItems []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	} `json:"workItems"`
}

// workItemsResponse is the response from the batch work items endpoint.
type workItemsResponse struct {
	Value []WorkItem `json:"value"`
}

// ListWorkItems returns all open work items assigned to the current user (@Me).
func (c *Client) ListWorkItems() ([]WorkItem, error) {
	query := wiqlRequest{
		Query: `SELECT [System.Id],[System.Title],[System.State],[System.AssignedTo],[System.WorkItemType],[System.ChangedDate]
FROM WorkItems
WHERE [System.AssignedTo] = @Me
AND [System.State] <> 'Closed'
AND [System.State] <> 'Resolved'
AND [System.State] <> 'Done'
ORDER BY [System.ChangedDate] DESC`,
	}

	wiqlURL := fmt.Sprintf("%s/_apis/wit/wiql?api-version=%s", c.projectURL(), apiVersion)
	var wiqlResp wiqlResponse
	if err := c.post(wiqlURL, query, &wiqlResp); err != nil {
		return nil, fmt.Errorf("azure list: WIQL query failed: %w", err)
	}

	if len(wiqlResp.WorkItems) == 0 {
		return nil, nil
	}

	ids := make([]string, len(wiqlResp.WorkItems))
	for i, wi := range wiqlResp.WorkItems {
		ids[i] = fmt.Sprintf("%d", wi.ID)
	}
	idsStr := strings.Join(ids, ",")

	batchURL := fmt.Sprintf("%s/_apis/wit/workitems?ids=%s&fields=System.Id,System.Title,System.State,System.AssignedTo,System.WorkItemType,System.ChangedDate&api-version=%s",
		c.orgURL(), idsStr, apiVersion)

	var batchResp workItemsResponse
	if err := c.get(batchURL, &batchResp); err != nil {
		return nil, fmt.Errorf("azure list: batch fetch failed: %w", err)
	}

	for i := range batchResp.Value {
		batchResp.Value[i].WebURL = fmt.Sprintf("%s/%s/_workitems/edit/%d",
			c.baseURL, c.org, batchResp.Value[i].ID)
	}

	return batchResp.Value, nil
}

// ListWorkItemsChangedAfter returns work items assigned to @Me that changed after since.
// Pass zero time to fall back to ListWorkItems (no date filter).
func (c *Client) ListWorkItemsChangedAfter(since time.Time) ([]WorkItem, error) {
	if since.IsZero() {
		return c.ListWorkItems()
	}
	sinceStr := since.UTC().Format("2006-01-02")
	query := wiqlRequest{
		Query: `SELECT [System.Id],[System.Title],[System.State],[System.AssignedTo],[System.WorkItemType],[System.ChangedDate]
FROM WorkItems
WHERE [System.AssignedTo] = @Me
AND [System.ChangedDate] > '` + sinceStr + `'
ORDER BY [System.ChangedDate] DESC`,
	}

	wiqlURL := fmt.Sprintf("%s/_apis/wit/wiql?api-version=%s", c.projectURL(), apiVersion)
	var wiqlResp wiqlResponse
	if err := c.post(wiqlURL, query, &wiqlResp); err != nil {
		return nil, fmt.Errorf("azure list: WIQL query failed: %w", err)
	}

	if len(wiqlResp.WorkItems) == 0 {
		return nil, nil
	}

	ids := make([]string, len(wiqlResp.WorkItems))
	for i, wi := range wiqlResp.WorkItems {
		ids[i] = fmt.Sprintf("%d", wi.ID)
	}

	batchURL := fmt.Sprintf("%s/_apis/wit/workitems?ids=%s&fields=System.Id,System.Title,System.State,System.AssignedTo,System.WorkItemType,System.ChangedDate&api-version=%s",
		c.orgURL(), strings.Join(ids, ","), apiVersion)

	var batchResp workItemsResponse
	if err := c.get(batchURL, &batchResp); err != nil {
		return nil, fmt.Errorf("azure list: batch fetch failed: %w", err)
	}

	for i := range batchResp.Value {
		batchResp.Value[i].WebURL = fmt.Sprintf("%s/%s/_workitems/edit/%d",
			c.baseURL, c.org, batchResp.Value[i].ID)
	}

	return batchResp.Value, nil
}
