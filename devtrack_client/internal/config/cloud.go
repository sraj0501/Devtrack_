package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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
