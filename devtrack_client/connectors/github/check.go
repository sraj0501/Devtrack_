package github

import (
	"fmt"
	"os"
)

// rateLimitResponse matches the GitHub rate limit API shape.
type rateLimitResponse struct {
	Resources struct {
		Core struct {
			Limit     int `json:"limit"`
			Remaining int `json:"remaining"`
			Reset     int `json:"reset"`
		} `json:"core"`
	} `json:"resources"`
}

// Check verifies GitHub connectivity and prints the authenticated user and rate limit.
func Check() error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is not set\nSet it in your .env: GITHUB_TOKEN=ghp_...")
	}

	c, err := NewClient()
	if err != nil {
		return err
	}

	// Fetch authenticated user
	var user struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := c.do("/user", &user); err != nil {
		return fmt.Errorf("github check: authentication failed: %w", err)
	}

	// Fetch rate limit
	var rl rateLimitResponse
	if err := c.do("/rate_limit", &rl); err != nil {
		// Non-fatal — connectivity confirmed, rate limit is informational
		fmt.Printf("GitHub: connected as %s (%s)\n", user.Login, user.Name)
		return nil
	}

	fmt.Printf("GitHub: connected\n")
	fmt.Printf("  User:          %s (%s)\n", user.Login, user.Name)
	fmt.Printf("  API rate limit: %d/%d remaining\n",
		rl.Resources.Core.Remaining, rl.Resources.Core.Limit)
	return nil
}

// CheckWithToken verifies connectivity using an explicit token (for testing).
func CheckWithToken(token string) error {
	orig := os.Getenv("GITHUB_TOKEN")
	_ = os.Setenv("GITHUB_TOKEN", token)
	err := Check()
	_ = os.Setenv("GITHUB_TOKEN", orig)
	return err
}
