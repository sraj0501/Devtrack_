package azure

import (
	"fmt"
	"os"
)

// projectsResponse is the response from the projects list endpoint.
type projectsResponse struct {
	Count int `json:"count"`
	Value []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"value"`
}

// Check verifies Azure DevOps connectivity and prints org/project info.
func Check() error {
	for _, v := range []string{"AZURE_DEVOPS_PAT", "AZURE_ORG", "AZURE_PROJECT"} {
		if os.Getenv(v) == "" {
			return fmt.Errorf("%s is not set\nSet it in your .env file", v)
		}
	}

	c, err := NewClient()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/_apis/projects?api-version=%s", c.orgURL(), apiVersion)
	var projs projectsResponse
	if err := c.get(url, &projs); err != nil {
		return fmt.Errorf("azure check: authentication failed: %w", err)
	}

	fmt.Printf("Azure DevOps: connected\n")
	fmt.Printf("  Org:        %s\n", c.org)
	fmt.Printf("  Project:    %s\n", c.project)
	fmt.Printf("  Projects in org: %d\n", projs.Count)

	// Show that the configured project is accessible
	found := false
	for _, p := range projs.Value {
		if p.Name == c.project {
			found = true
			fmt.Printf("  Project state: %s\n", p.State)
			break
		}
	}
	if !found && projs.Count > 0 {
		fmt.Printf("  Warning: project '%s' not found in org. Available projects:\n", c.project)
		for _, p := range projs.Value {
			fmt.Printf("    - %s\n", p.Name)
		}
	}

	return nil
}
