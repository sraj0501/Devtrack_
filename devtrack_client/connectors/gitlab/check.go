package gitlab

import "fmt"

// Check verifies GitLab connectivity and prints the authenticated user info.
func (c *Client) Check() error {
	var user AuthenticatedUser
	if err := c.do("/user", &user); err != nil {
		return fmt.Errorf("gitlab check: authentication failed: %w", err)
	}

	fmt.Printf("GitLab: connected\n")
	fmt.Printf("  Instance:  %s\n", c.baseURL)
	fmt.Printf("  User:      %s (%s)\n", user.Username, user.Name)
	if user.Email != "" {
		fmt.Printf("  Email:     %s\n", user.Email)
	}
	return nil
}
