package gitlab

import (
	"fmt"
	"os"
)

// Check verifies GitLab connectivity and prints the authenticated user info.
func Check() error {
	token := os.Getenv("GITLAB_PAT")
	if token == "" {
		return fmt.Errorf("GITLAB_PAT is not set\nSet it in your .env: GITLAB_PAT=glpat-...")
	}

	c, err := NewClient()
	if err != nil {
		return err
	}

	var user AuthenticatedUser
	if err := c.do("/user", &user); err != nil {
		return fmt.Errorf("gitlab check: authentication failed: %w", err)
	}

	gitlabURL := os.Getenv("GITLAB_URL")
	if gitlabURL == "" {
		gitlabURL = defaultGitLabURL
	}

	fmt.Printf("GitLab: connected\n")
	fmt.Printf("  Instance:  %s\n", gitlabURL)
	fmt.Printf("  User:      %s (%s)\n", user.Username, user.Name)
	if user.Email != "" {
		fmt.Printf("  Email:     %s\n", user.Email)
	}
	return nil
}
