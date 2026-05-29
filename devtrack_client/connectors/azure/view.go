package azure

import (
	"fmt"
	"strings"
)

// ViewWorkItem fetches a single Azure DevOps work item by ID.
func ViewWorkItem(id int) (*WorkItem, error) {
	c, err := NewClient()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/_apis/wit/workitems/%d?$expand=all&api-version=%s",
		c.orgURL(), id, apiVersion)

	var item WorkItem
	if err := c.get(url, &item); err != nil {
		return nil, fmt.Errorf("azure view: %w", err)
	}
	item.WebURL = fmt.Sprintf("%s/%s/_workitems/edit/%d", c.baseURL, c.org, id)
	return &item, nil
}

// FormatWorkItem returns a human-readable multi-line summary of a work item.
func FormatWorkItem(w *WorkItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Azure DevOps #%d — %s\n", w.ID, w.Title())
	fmt.Fprintf(&b, "Type:       %s\n", w.WorkItemType())
	fmt.Fprintf(&b, "State:      %s\n", w.State())
	fmt.Fprintf(&b, "Assigned:   %s\n", w.AssignedTo())
	fmt.Fprintf(&b, "Updated:    %s\n", w.UpdatedAt())
	if w.WebURL != "" {
		fmt.Fprintf(&b, "URL:        %s\n", w.WebURL)
	}
	// Description / repro steps
	for _, field := range []string{"System.Description", "Microsoft.VSTS.TCM.ReproSteps"} {
		if v, ok := w.Fields[field]; ok {
			if s, ok := v.(string); ok && s != "" {
				// Strip basic HTML tags for terminal display
				clean := stripHTML(s)
				if len(clean) > 500 {
					clean = clean[:500] + "..."
				}
				fmt.Fprintf(&b, "\n%s\n", clean)
				break
			}
		}
	}
	return b.String()
}

// stripHTML removes common HTML tags for terminal display.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, ch := range s {
		switch {
		case ch == '<':
			inTag = true
		case ch == '>':
			inTag = false
		case !inTag:
			b.WriteRune(ch)
		}
	}
	// Collapse multiple whitespace
	result := b.String()
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(result)
}
