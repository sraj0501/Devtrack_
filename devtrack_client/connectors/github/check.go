package github

import "fmt"

// rateLimitResponse matches the GitHub rate limit API shape.
type rateLimitResponse struct {
	Resources struct {
		Core struct {
			Limit     int `json:"limit"`
			Remaining int `json:"remaining"`
		} `json:"core"`
	} `json:"resources"`
}

// Check verifies GitHub connectivity and prints the authenticated user and rate limit.
func (c *Client) Check() error {
	var user AuthenticatedUser
	if err := c.do("/user", &user); err != nil {
		return fmt.Errorf("github check: authentication failed: %w", err)
	}

	var rl rateLimitResponse
	if err := c.do("/rate_limit", &rl); err != nil {
		fmt.Printf("GitHub: connected as %s (%s)\n", user.Login, user.Name)
		return nil
	}

	fmt.Printf("GitHub: connected\n")
	fmt.Printf("  User:           %s (%s)\n", user.Login, user.Name)
	fmt.Printf("  API rate limit: %d/%d remaining\n",
		rl.Resources.Core.Remaining, rl.Resources.Core.Limit)
	return nil
}
