package main

// cloud_cli.go — CLI command handlers for "devtrack cloud ..." commands.
// The CloudConfig type and cloud credential functions live in internal/config.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// handleCloud dispatches devtrack cloud <subcommand>
func (cli *CLI) handleCloud() error {
	args := os.Args
	sub := ""
	if len(args) > 2 {
		sub = args[2]
	}
	switch sub {
	case "login":
		return cli.handleCloudLogin(args[3:])
	case "logout":
		return cli.handleCloudLogout()
	case "status":
		return cli.handleCloudStatus()
	default:
		fmt.Println("Usage:")
		fmt.Println("  devtrack cloud login --url URL --key KEY   Connect to a remote DevTrack server")
		fmt.Println("  devtrack cloud status                      Ping the remote server")
		fmt.Println("  devtrack cloud logout                      Disconnect and revert to managed mode")
		return nil
	}
}

func (cli *CLI) handleCloudLogin(args []string) error {
	var url, key string
	for i := 0; i+1 < len(args); i++ {
		switch args[i] {
		case "--url":
			url = args[i+1]
		case "--key":
			key = args[i+1]
		}
	}
	if url == "" {
		return fmt.Errorf("--url is required (e.g. --url https://myserver.com)")
	}
	if key == "" {
		return fmt.Errorf("--key is required (e.g. --key your-api-key)")
	}
	url = strings.TrimRight(url, "/")

	fmt.Printf("Connecting to %s …\n", url)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url+"/health", nil)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("X-DevTrack-API-Key", key)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach server at %s: %w\nCheck the URL and that the server is running.", url, err)
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case 401, 403:
		return fmt.Errorf("server rejected the API key (HTTP %d) — check --key", resp.StatusCode)
	case 200:
		// ok
	default:
		return fmt.Errorf("server returned HTTP %d — expected 200", resp.StatusCode)
	}

	c := &CloudConfig{Mode: "cloud", URL: url, APIKey: key}
	if err := SaveCloudConfig(c); err != nil {
		return fmt.Errorf("failed to save cloud config: %w", err)
	}

	fmt.Printf("✓ Connected to %s\n", url)
	fmt.Println("  Credentials saved to ~/.devtrack/cloud.json (mode 0600)")
	fmt.Println()
	fmt.Println("  devtrack start         — start Go daemon in cloud mode (no local Python needed)")
	fmt.Println("  devtrack cloud status  — verify connection at any time")
	return nil
}

func (cli *CLI) handleCloudLogout() error {
	if !IsCloudMode() {
		fmt.Println("Not currently in cloud mode.")
		return nil
	}
	c, _ := LoadCloudConfig()
	url := ""
	if c != nil {
		url = c.URL
	}
	if err := ClearCloudConfig(); err != nil {
		return fmt.Errorf("failed to remove cloud config: %w", err)
	}
	fmt.Printf("✓ Disconnected from %s\n", url)
	fmt.Println("  Cloud credentials removed. Daemon will use DEVTRACK_SERVER_MODE from .env (default: managed).")
	return nil
}

func (cli *CLI) handleCloudStatus() error {
	if !IsCloudMode() {
		url := os.Getenv("DEVTRACK_SERVER_URL")
		mode := os.Getenv("DEVTRACK_SERVER_MODE")
		if url != "" {
			fmt.Printf("External server (env vars): %s\n", url)
			fmt.Println("Tip: run 'devtrack cloud login --url URL --key KEY' to store credentials in cloud.json.")
		} else if mode == "external" {
			fmt.Println("External mode, no DEVTRACK_SERVER_URL set — defaulting to a server on this machine (localhost).")
			fmt.Println("To target a server on another host, set DEVTRACK_SERVER_URL in .env or run: devtrack cloud login --url URL --key KEY")
		} else {
			fmt.Println("Running in managed mode (daemon spawns Python locally).")
			fmt.Println("To connect to a remote server: devtrack cloud login --url URL --key KEY")
		}
		return nil
	}

	c, _ := LoadCloudConfig()
	fmt.Printf("Cloud server: %s\n", c.URL)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Health + latency
	start := time.Now()
	req, _ := http.NewRequest("GET", c.URL+"/health", nil)
	req.Header.Set("X-DevTrack-API-Key", c.APIKey)
	resp, err := httpClient.Do(req)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		fmt.Printf("  Health:  ✗ unreachable (%v)\n", err)
		return nil
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		fmt.Printf("  Health:  ✓ up (%dms)\n", latencyMs)
	} else {
		fmt.Printf("  Health:  ✗ HTTP %d\n", resp.StatusCode)
	}

	// Version
	req2, _ := http.NewRequest("GET", c.URL+"/version", nil)
	req2.Header.Set("X-DevTrack-API-Key", c.APIKey)
	if resp2, err2 := httpClient.Do(req2); err2 == nil && resp2.StatusCode == 200 {
		var body map[string]interface{}
		json.NewDecoder(resp2.Body).Decode(&body)
		resp2.Body.Close()
		if v, ok := body["version"]; ok {
			fmt.Printf("  Version: %v\n", v)
		}
	}

	// Key preview (never print the full key)
	keyLen := len(c.APIKey)
	preview := c.APIKey
	if keyLen > 8 {
		preview = c.APIKey[:8] + "…"
	}
	fmt.Printf("  API key: %s\n", preview)
	fmt.Printf("  Config:  ~/.devtrack/cloud.json\n")
	return nil
}

// handleTUI launches the Bubble Tea TUI dashboard (implemented in tui.go).
func (cli *CLI) handleTUI() error {
	return RunTUI()
}
