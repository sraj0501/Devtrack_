package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// WorkspaceConfig represents a single monitored repo in workspaces.yaml.
//
// API keys / tokens (GITHUB_TOKEN, GITLAB_PAT, AZURE_DEVOPS_PAT) are the
// only connector values that live in .env — everything else is here.
type WorkspaceConfig struct {
	Name           string   `yaml:"name"`
	Path           string   `yaml:"path"`
	PMPlatform     string   `yaml:"pm_platform"` // "azure" | "gitlab" | "github" | "jira" | "none" | ""
	PMProject      string   `yaml:"pm_project"`  // owner/repo (GitHub), group/project or ID (GitLab), project name (Azure)
	PMOrg          string   `yaml:"pm_org"`      // Azure: org name (e.g. "mycompany"); not used by GitHub/GitLab
	PMUsername     string   `yaml:"pm_username"` // assignee filter: GitHub login, GitLab username, Azure email
	PMAPIURL       string   `yaml:"pm_api_url"`  // self-hosted override: GitHub Enterprise, GitLab self-hosted, Azure DevOps Server
	Enabled        bool     `yaml:"enabled"`
	IgnoreBranches []string `yaml:"ignore_branches"`
	Tags           []string `yaml:"tags"`
	// Azure-specific sprint / area settings
	PMAssignee      string `yaml:"pm_assignee"`       // Azure: assigned_to display name
	PMIterationPath string `yaml:"pm_iteration_path"` // Azure: sprint path (e.g. "MyProject\\Sprint 5")
	PMAreaPath      string `yaml:"pm_area_path"`      // Azure: area path (e.g. "MyProject\\Backend")
	PMMilestone     int    `yaml:"pm_milestone"`      // GitHub: milestone number; GitLab: milestone_id
	// SkipIssues marks this workspace as code-only: it is excluded from
	// "devtrack issues", ticket sync, and the commit-time ticket picker.
	// Use when one repo is tracked in two platforms (e.g. GitHub for code,
	// Azure DevOps for PM) to avoid showing duplicate tickets.
	SkipIssues bool `yaml:"skip_issues"`
}

// WorkspacesConfig is the top-level structure of workspaces.yaml
type WorkspacesConfig struct {
	Version    string            `yaml:"version"`
	Workspaces []WorkspaceConfig `yaml:"workspaces"`
}

// LoadWorkspacesConfig loads workspaces.yaml if it exists.
// Returns (nil, nil) when the file does not exist (backward compat: single-repo mode).
func LoadWorkspacesConfig() (*WorkspacesConfig, error) {
	path := GetWorkspacesFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to check workspaces file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspaces file: %w", err)
	}

	cfg := &WorkspacesConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse workspaces file: %w", err)
	}

	// Expand ~ in paths
	for i := range cfg.Workspaces {
		cfg.Workspaces[i].Path = expandWorkspacePath(cfg.Workspaces[i].Path)
	}

	return cfg, nil
}

// ResolveWorkspaceForPath returns the enabled workspace whose Path best matches
// (longest prefix of) the given filesystem path. Returns (nil, nil) when there
// is no workspaces.yaml or no matching workspace (single-repo / unconfigured mode).
func ResolveWorkspaceForPath(p string) (*WorkspaceConfig, error) {
	cfg, err := LoadWorkspacesConfig()
	if err != nil || cfg == nil {
		return nil, err
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	abs = filepath.Clean(abs)

	var best *WorkspaceConfig
	bestLen := -1
	for i := range cfg.Workspaces {
		ws := &cfg.Workspaces[i]
		if !ws.Enabled {
			continue
		}
		wp := filepath.Clean(ws.Path)
		if abs == wp || strings.HasPrefix(abs, wp+string(filepath.Separator)) {
			// Prefer longer prefix match. On a tie, prefer a workspace that is
			// not skip_issues so the commit-time ticket picker uses the PM
			// platform rather than the code-only platform.
			if len(wp) > bestLen || (len(wp) == bestLen && best != nil && best.SkipIssues && !ws.SkipIssues) {
				best = ws
				bestLen = len(wp)
			}
		}
	}
	return best, nil
}

// expandWorkspacePath expands ~ to home directory in workspace paths
func expandWorkspacePath(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// ExpandWorkspacePath is the exported version of expandWorkspacePath.
func ExpandWorkspacePath(path string) string { return expandWorkspacePath(path) }

// Save writes the WorkspacesConfig back to the workspaces.yaml file.
func (wc *WorkspacesConfig) Save() error {
	path := GetWorkspacesFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory for workspaces file: %w", err)
	}

	// Normalize workspace paths to forward slashes before marshalling.
	// yaml.Marshal double-quotes strings containing backslashes, which causes
	// the YAML parser to interpret sequences like \u as Unicode escapes.
	// Forward slashes are valid path separators on Windows.
	normalized := *wc
	normalized.Workspaces = make([]WorkspaceConfig, len(wc.Workspaces))
	copy(normalized.Workspaces, wc.Workspaces)
	for i := range normalized.Workspaces {
		normalized.Workspaces[i].Path = filepath.ToSlash(normalized.Workspaces[i].Path)
	}

	data, err := yaml.Marshal(&normalized)
	if err != nil {
		return fmt.Errorf("failed to marshal workspaces config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspaces file: %w", err)
	}

	return nil
}

// GetEnabledWorkspaces returns all enabled workspace configs
func (wc *WorkspacesConfig) GetEnabledWorkspaces() []WorkspaceConfig {
	var enabled []WorkspaceConfig
	for _, ws := range wc.Workspaces {
		if ws.Enabled {
			enabled = append(enabled, ws)
		}
	}
	return enabled
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
