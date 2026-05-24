package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CloudConfig holds cloud credentials persisted to ~/.devtrack/cloud.json
type CloudConfig struct {
	Mode   string `json:"mode"`    // always "cloud"
	URL    string `json:"url"`     // e.g. "https://myserver.com"
	APIKey string `json:"api_key"` // stored chmod 0600
}

func cloudConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devtrack", "cloud.json")
}

// LoadCloudConfig reads cloud credentials from ~/.devtrack/cloud.json.
func LoadCloudConfig() (*CloudConfig, error) {
	data, err := os.ReadFile(cloudConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg CloudConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveCloudConfig writes cloud credentials to ~/.devtrack/cloud.json (chmod 0600).
func SaveCloudConfig(cfg *CloudConfig) error {
	path := cloudConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ClearCloudConfig removes the cloud credentials file.
func ClearCloudConfig() error {
	err := os.Remove(cloudConfigPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsCloudMode reports whether ~/.devtrack/cloud.json exists with mode=cloud.
func IsCloudMode() bool {
	cfg, err := LoadCloudConfig()
	return err == nil && cfg.Mode == "cloud"
}

// GetCloudAPIKey returns the API key: cloud.json first, then DEVTRACK_API_KEY env var.
func GetCloudAPIKey() string {
	if cfg, err := LoadCloudConfig(); err == nil && cfg.APIKey != "" {
		return cfg.APIKey
	}
	return os.Getenv("DEVTRACK_API_KEY")
}

// GetCloudURL returns the server URL: cloud.json first, then DEVTRACK_SERVER_URL env var.
func GetCloudURL() string {
	if cfg, err := LoadCloudConfig(); err == nil && cfg.URL != "" {
		return cfg.URL
	}
	return os.Getenv("DEVTRACK_SERVER_URL")
}

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

	cfg := &CloudConfig{Mode: "cloud", URL: url, APIKey: key}
	if err := SaveCloudConfig(cfg); err != nil {
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
	cfg, _ := LoadCloudConfig()
	url := ""
	if cfg != nil {
		url = cfg.URL
	}
	if err := ClearCloudConfig(); err != nil {
		return fmt.Errorf("failed to remove cloud config: %w", err)
	}
	fmt.Printf("✓ Disconnected from %s\n", url)
	fmt.Println("  Reverted to managed mode — devtrack start will spawn a local Python backend.")
	return nil
}

func (cli *CLI) handleCloudStatus() error {
	if !IsCloudMode() {
		url := os.Getenv("DEVTRACK_SERVER_URL")
		mode := os.Getenv("DEVTRACK_SERVER_MODE")
		if url != "" && mode == "external" {
			fmt.Printf("External mode (env vars): %s\n", url)
			fmt.Println("Tip: run 'devtrack cloud login --url URL --key KEY' for managed credentials.")
		} else {
			fmt.Println("Not in cloud mode. Run 'devtrack cloud login --url URL --key KEY' to connect.")
		}
		return nil
	}

	cfg, _ := LoadCloudConfig()
	fmt.Printf("Cloud server: %s\n", cfg.URL)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Health + latency
	start := time.Now()
	req, _ := http.NewRequest("GET", cfg.URL+"/health", nil)
	req.Header.Set("X-DevTrack-API-Key", cfg.APIKey)
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
	req2, _ := http.NewRequest("GET", cfg.URL+"/version", nil)
	req2.Header.Set("X-DevTrack-API-Key", cfg.APIKey)
	if resp2, err2 := httpClient.Do(req2); err2 == nil && resp2.StatusCode == 200 {
		var body map[string]interface{}
		json.NewDecoder(resp2.Body).Decode(&body)
		resp2.Body.Close()
		if v, ok := body["version"]; ok {
			fmt.Printf("  Version: %v\n", v)
		}
	}

	// Key preview (never print the full key)
	keyLen := len(cfg.APIKey)
	preview := cfg.APIKey
	if keyLen > 8 {
		preview = cfg.APIKey[:8] + "…"
	}
	fmt.Printf("  API key: %s\n", preview)
	fmt.Printf("  Config:  ~/.devtrack/cloud.json\n")
	return nil
}

// handleTUI launches the Bubble Tea TUI dashboard (implemented in tui.go).
func (cli *CLI) handleTUI() error {
	return RunTUI()
}
