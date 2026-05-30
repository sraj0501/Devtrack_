package azure

import "fmt"

// projectsResponse is the response from the projects list endpoint.
type projectsResponse struct {
	Count int `json:"count"`
	Value []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"value"`
}

// Check verifies Azure DevOps connectivity and prints org/project info.
func (c *Client) Check() error {
	url := fmt.Sprintf("%s/_apis/projects?api-version=%s", c.orgURL(), apiVersion)
	var projs projectsResponse
	if err := c.get(url, &projs); err != nil {
		return fmt.Errorf("azure check: authentication failed: %w", err)
	}

	fmt.Printf("Azure DevOps: connected\n")
	fmt.Printf("  Org:     %s\n", c.org)
	fmt.Printf("  Project: %s\n", c.project)
	fmt.Printf("  Projects in org: %d\n", projs.Count)

	found := false
	for _, p := range projs.Value {
		if p.Name == c.project {
			found = true
			fmt.Printf("  Project state: %s\n", p.State)
			break
		}
	}
	if !found && projs.Count > 0 {
		fmt.Printf("  Warning: project %q not found in org. Available:\n", c.project)
		for _, p := range projs.Value {
			fmt.Printf("    - %s\n", p.Name)
		}
	}
	return nil
}
