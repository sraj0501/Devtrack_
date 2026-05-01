package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Version      string              `yaml:"version"`
	Repositories []RepositoryConfig  `yaml:"repositories"`
	Settings     Settings            `yaml:"settings"`
	Integrations IntegrationSettings `yaml:"integrations"`
}

// RepositoryConfig represents a single Git repository configuration
type RepositoryConfig struct {
	Name    string   `yaml:"name"`
	Path    string   `yaml:"path"`
	Enabled bool     `yaml:"enabled"`
	Project string   `yaml:"project"`
	Ignore  []string `yaml:"ignore"` // Branches or paths to ignore
}

// Settings contains general application settings
type Settings struct {
	PromptInterval int                `yaml:"prompt_interval"` // Minutes between prompts
	WorkHoursOnly  bool               `yaml:"work_hours_only"` // Only trigger during work hours
	WorkStartHour  int                `yaml:"work_start_hour"` // Start of work hours (24h format)
	WorkEndHour    int                `yaml:"work_end_hour"`   // End of work hours (24h format)
	Timezone       string             `yaml:"timezone"`        // Timezone for work hours
	LogLevel       string             `yaml:"log_level"`       // debug, info, warn, error
	AutoSync       bool               `yaml:"auto_sync"`       // Automatically sync with APIs
	Notifications  NotificationConfig `yaml:"notifications"`   // Notification settings
}

// NotificationConfig contains notification and output settings
type NotificationConfig struct {
	OutputType       string            `yaml:"output_type"`        // "email", "teams", "both"
	Email            EmailOutputConfig `yaml:"email"`              // Email output settings
	Teams            TeamsOutputConfig `yaml:"teams"`              // Teams output settings
	DailyReportTime  string            `yaml:"daily_report_time"`  // Time to send daily report (HH:MM)
	WeeklyReportDay  string            `yaml:"weekly_report_day"`  // Day for weekly report
	SendOnTrigger    bool              `yaml:"send_on_trigger"`    // Send notification on each trigger
	SendDailySummary bool              `yaml:"send_daily_summary"` // Send daily summary
}

// EmailOutputConfig contains email-specific settings
type EmailOutputConfig struct {
	Enabled      bool     `yaml:"enabled"`
	ToAddresses  []string `yaml:"to_addresses"`  // Recipient email addresses
	CCAddresses  []string `yaml:"cc_addresses"`  // CC email addresses
	Subject      string   `yaml:"subject"`       // Email subject template
	ManagerEmail string   `yaml:"manager_email"` // Manager's email for reports
}

// TeamsOutputConfig contains Teams-specific settings
type TeamsOutputConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ChannelID   string `yaml:"channel_id"`   // Teams channel ID
	ChannelName string `yaml:"channel_name"` // Teams channel name (for display)
	ChatID      string `yaml:"chat_id"`      // Teams chat ID (for 1-on-1)
	ChatType    string `yaml:"chat_type"`    // "channel" or "chat"
	WebhookURL  string `yaml:"webhook_url"`  // Incoming webhook URL
	MentionUser bool   `yaml:"mention_user"` // Mention user in messages
}

// IntegrationSettings contains API integration settings
type IntegrationSettings struct {
	AzureDevOps AzureDevOpsConfig `yaml:"azure_devops"`
	GitHub      GitHubConfig      `yaml:"github"`
	JIRA        JIRAConfig        `yaml:"jira"`
}

// AzureDevOpsConfig represents Azure DevOps settings
type AzureDevOpsConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Organization string `yaml:"organization"`
	Project      string `yaml:"project"`
	PAT          string `yaml:"pat"` // Personal Access Token (should be in env var)
}

// GitHubConfig represents GitHub settings
type GitHubConfig struct {
	Enabled bool   `yaml:"enabled"`
	Owner   string `yaml:"owner"`
	Repo    string `yaml:"repo"`
	Token   string `yaml:"token"` // Should be in env var
}

// JIRAConfig represents JIRA settings
type JIRAConfig struct {
	Enabled  bool   `yaml:"enabled"`
	URL      string `yaml:"url"`
	Project  string `yaml:"project"`
	Username string `yaml:"username"`
	APIToken string `yaml:"api_token"` // Should be in env var
}

// GetConfigPath returns the path to the configuration file
func GetConfigPath() string {
	return filepath.Join(GetConfigDirPath(), GetConfigFileName())
}

// LoadConfig loads the configuration from the YAML file
func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return CreateDefaultConfig()
	} else if err != nil {
		return nil, fmt.Errorf("failed to check config file: %w", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	applyConfigDefaults(config)
	return config, nil
}

// CreateDefaultConfig creates a default configuration file.
func CreateDefaultConfig() (*Config, error) {
	config := &Config{
		Version:      GetDevTrackVersion(),
		Repositories: []RepositoryConfig{},
		Settings: Settings{
			PromptInterval: GetPromptInterval(),
			WorkHoursOnly:  GetWorkHoursOnly(),
			WorkStartHour:  GetWorkStartHour(),
			WorkEndHour:    GetWorkEndHour(),
			Timezone:       GetTimezone(),
			LogLevel:       GetLogLevel(),
			AutoSync:       GetAutoSync(),
			Notifications: NotificationConfig{
				OutputType:       GetOutputType(),
				DailyReportTime:  GetDailyReportTime(),
				WeeklyReportDay:  GetWeeklyReportDay(),
				SendOnTrigger:    GetSendOnTrigger(),
				SendDailySummary: GetSendDailySummary(),
				Email: EmailOutputConfig{
					Enabled:      false,
					ToAddresses:  GetEmailToAddresses(),
					CCAddresses:  GetEmailCCAddresses(),
					Subject:      GetEmailSubject(),
					ManagerEmail: GetEmailManager(),
				},
				Teams: TeamsOutputConfig{
					Enabled:     false,
					ChannelID:   GetTeamsChannelID(),
					ChannelName: GetTeamsChannelName(),
					ChatID:      GetTeamsChatID(),
					ChatType:    GetTeamsChatType(),
					WebhookURL:  GetTeamsWebhookURL(),
					MentionUser: GetTeamsMentionUser(),
				},
			},
		},
		Integrations: IntegrationSettings{
			AzureDevOps: AzureDevOpsConfig{
				Enabled:      true,
				Organization: "",
				Project:      "",
				PAT:          "${AZURE_DEVOPS_PAT}",
			},
			GitHub: GitHubConfig{
				Enabled: true,
				Owner:   "",
				Repo:    "",
				Token:   "${GITHUB_TOKEN}",
			},
			JIRA: JIRAConfig{
				Enabled:  false,
				URL:      "",
				Project:  "",
				Username: "",
				APIToken: "${JIRA_API_TOKEN}",
			},
		},
	}

	if err := config.Save(); err != nil {
		return nil, err
	}

	fmt.Printf("✓ Created default configuration at: %s\n", GetConfigPath())
	return config, nil
}

// Save persists the configuration to disk.
func (c *Config) Save() error {
	configPath := GetConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func applyConfigDefaults(config *Config) {
	config.Version = GetDevTrackVersion()

	if config.Repositories == nil {
		config.Repositories = []RepositoryConfig{}
	}

	config.Settings.PromptInterval = GetPromptInterval()
	config.Settings.WorkHoursOnly = GetWorkHoursOnly()
	config.Settings.WorkStartHour = GetWorkStartHour()
	config.Settings.WorkEndHour = GetWorkEndHour()
	config.Settings.Timezone = GetTimezone()
	config.Settings.LogLevel = GetLogLevel()
	config.Settings.AutoSync = GetAutoSync()
	config.Settings.Notifications.OutputType = GetOutputType()
	config.Settings.Notifications.DailyReportTime = GetDailyReportTime()
	config.Settings.Notifications.WeeklyReportDay = GetWeeklyReportDay()
	config.Settings.Notifications.SendOnTrigger = GetSendOnTrigger()
	config.Settings.Notifications.SendDailySummary = GetSendDailySummary()
	config.Settings.Notifications.Email.ToAddresses = GetEmailToAddresses()
	config.Settings.Notifications.Email.CCAddresses = GetEmailCCAddresses()
	config.Settings.Notifications.Email.ManagerEmail = GetEmailManager()
	config.Settings.Notifications.Email.Subject = GetEmailSubject()
	config.Settings.Notifications.Teams.ChannelID = GetTeamsChannelID()
	config.Settings.Notifications.Teams.ChannelName = GetTeamsChannelName()
	config.Settings.Notifications.Teams.ChatID = GetTeamsChatID()
	config.Settings.Notifications.Teams.ChatType = GetTeamsChatType()
	config.Settings.Notifications.Teams.WebhookURL = GetTeamsWebhookURL()
	config.Settings.Notifications.Teams.MentionUser = GetTeamsMentionUser()

}

// AddRepository adds a new repository to the configuration
func (c *Config) AddRepository(name, path, project string) error {
	// Check if repository already exists
	for _, repo := range c.Repositories {
		if repo.Path == path {
			return fmt.Errorf("repository already configured: %s", path)
		}
	}

	// Verify it's a git repository
	if !IsGitRepository(path) {
		return fmt.Errorf("not a git repository: %s", path)
	}

	c.Repositories = append(c.Repositories, RepositoryConfig{
		Name:    name,
		Path:    path,
		Enabled: true,
		Project: project,
		Ignore:  []string{},
	})

	return c.Save()
}

// RemoveRepository removes a repository from the configuration
func (c *Config) RemoveRepository(path string) error {
	for i, repo := range c.Repositories {
		if repo.Path == path {
			c.Repositories = append(c.Repositories[:i], c.Repositories[i+1:]...)
			return c.Save()
		}
	}
	return fmt.Errorf("repository not found: %s", path)
}

// GetEnabledRepositories returns all enabled repositories
func (c *Config) GetEnabledRepositories() []RepositoryConfig {
	var enabled []RepositoryConfig
	for _, repo := range c.Repositories {
		if repo.Enabled {
			enabled = append(enabled, repo)
		}
	}
	return enabled
}
