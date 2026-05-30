package main

// config_shim.go — forwarding aliases from package main to internal/config.
// This file exists so that the 30+ caller files in package main do not need
// to be changed. All logic lives in internal/config; this file just re-exports.

import (
	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// ── Type aliases ─────────────────────────────────────────────────────────────

type Config = cfg.Config
type RepositoryConfig = cfg.RepositoryConfig
type WorkspaceConfig = cfg.WorkspaceConfig
type WorkspacesConfig = cfg.WorkspacesConfig
type Settings = cfg.Settings
type NotificationConfig = cfg.NotificationConfig
type EmailOutputConfig = cfg.EmailOutputConfig
type TeamsOutputConfig = cfg.TeamsOutputConfig
type IntegrationSettings = cfg.IntegrationSettings
type AzureDevOpsConfig = cfg.AzureDevOpsConfig
type GitHubConfig = cfg.GitHubConfig
type JIRAConfig = cfg.JIRAConfig
type EnvConfig = cfg.EnvConfig
type CloudConfig = cfg.CloudConfig
type ServerMode = cfg.ServerMode

const (
	ServerModeManaged  = cfg.ServerModeManaged
	ServerModeExternal = cfg.ServerModeExternal
	devtrackConfFile   = cfg.DevtrackConfFile
)

// ── Function forwards — config.go ────────────────────────────────────────────

func LoadConfig() (*Config, error)                          { return cfg.LoadConfig() }
func LoadWorkspacesConfig() (*WorkspacesConfig, error)      { return cfg.LoadWorkspacesConfig() }
func CreateDefaultConfig() (*Config, error)                 { return cfg.CreateDefaultConfig() }
func GetConfigPath() string                                  { return cfg.GetConfigPath() }
func expandWorkspacePath(path string) string                { return cfg.ExpandWorkspacePath(path) }

// ── Function forwards — config_env.go ────────────────────────────────────────

func LoadEnvConfig() (*EnvConfig, error)                    { return cfg.LoadEnvConfig() }
func GetDevTrackDir() string                                 { return cfg.GetDevTrackDir() }
func GetIPCAddress() string                                  { return cfg.GetIPCAddress() }
func GetConfigFileName() string                              { return cfg.GetConfigFileName() }
func GetConfigDirPath() string                               { return cfg.GetConfigDirPath() }
func GetDatabaseFileName() string                            { return cfg.GetDatabaseFileName() }
func GetDatabaseDir() string                                 { return cfg.GetDatabaseDir() }
func GetDatabasePath() string                                { return cfg.GetDatabasePath() }
func GetPIDFileName() string                                 { return cfg.GetPIDFileName() }
func GetPIDDir() string                                      { return cfg.GetPIDDir() }
func GetPIDFilePath() string                                 { return cfg.GetPIDFilePath() }
func GetLogFileName() string                                 { return cfg.GetLogFileName() }
func GetLogDir() string                                      { return cfg.GetLogDir() }
func GetLogFilePath() string                                 { return cfg.GetLogFilePath() }
func GetLearningDirName() string                             { return cfg.GetLearningDirName() }
func GetLearningDirPath() string                             { return cfg.GetLearningDirPath() }
func GetCLIAppName() string                                  { return cfg.GetCLIAppName() }
func GetPromptInterval() int                                 { return cfg.GetPromptInterval() }
func GetWorkHoursOnly() bool                                 { return cfg.GetWorkHoursOnly() }
func GetWorkStartHour() int                                  { return cfg.GetWorkStartHour() }
func GetWorkEndHour() int                                    { return cfg.GetWorkEndHour() }
func GetTimezone() string                                    { return cfg.GetTimezone() }
func GetLogLevel() string                                    { return cfg.GetLogLevel() }
func GetAutoSync() bool                                      { return cfg.GetAutoSync() }
func GetOutputType() string                                  { return cfg.GetOutputType() }
func GetDailyReportTime() string                             { return cfg.GetDailyReportTime() }
func GetWeeklyReportDay() string                             { return cfg.GetWeeklyReportDay() }
func GetSendOnTrigger() bool                                 { return cfg.GetSendOnTrigger() }
func GetSendDailySummary() bool                              { return cfg.GetSendDailySummary() }
func GetEmailToAddresses() []string                          { return cfg.GetEmailToAddresses() }
func GetEmailCCAddresses() []string                          { return cfg.GetEmailCCAddresses() }
func GetEmailManager() string                                { return cfg.GetEmailManager() }
func GetEmailSubject() string                                { return cfg.GetEmailSubject() }
func GetTeamsChannelID() string                              { return cfg.GetTeamsChannelID() }
func GetTeamsChannelName() string                            { return cfg.GetTeamsChannelName() }
func GetTeamsChatID() string                                 { return cfg.GetTeamsChatID() }
func GetTeamsChatType() string                               { return cfg.GetTeamsChatType() }
func GetTeamsWebhookURL() string                             { return cfg.GetTeamsWebhookURL() }
func GetTeamsMentionUser() bool                              { return cfg.GetTeamsMentionUser() }
func GetLearningDefaultDays() int                            { return cfg.GetLearningDefaultDays() }
func GetDevTrackVersion() string                             { return cfg.GetDevTrackVersion() }
func GetDevTrackBuildDate() string                           { return cfg.GetDevTrackBuildDate() }
func GetIPCConnectTimeoutSecs() int                          { return cfg.GetIPCConnectTimeoutSecs() }
func GetWorkspacesFilePath() string                          { return cfg.GetWorkspacesFilePath() }
func IsWebhookEnabled() bool                                 { return cfg.IsWebhookEnabled() }
func GetGitHubToken() string                                 { return cfg.GetGitHubToken() }
func GetGitHubDefaultRepo() string                           { return cfg.GetGitHubDefaultRepo() }
func GetGitHubAssignee() string                              { return cfg.GetGitHubAssignee() }
func GetWebhookPort() int                                    { return cfg.GetWebhookPort() }
func GetHealthCheckIntervalSecs() int                        { return cfg.GetHealthCheckIntervalSecs() }
func GetHealthAutoRestartPython() bool                       { return cfg.GetHealthAutoRestartPython() }
func GetHealthAutoRestartWebhook() bool                      { return cfg.GetHealthAutoRestartWebhook() }
func GetHealthMaxRestartsPerHour() int                       { return cfg.GetHealthMaxRestartsPerHour() }
func GetQueueDrainIntervalSecs() int                         { return cfg.GetQueueDrainIntervalSecs() }
func GetQueueMaxRetries() int                                { return cfg.GetQueueMaxRetries() }
func GetQueueRetentionDays() int                             { return cfg.GetQueueRetentionDays() }
func GetDeferredCommitExpiryHours() int                      { return cfg.GetDeferredCommitExpiryHours() }
func IsTelegramEnabled() bool                                { return cfg.IsTelegramEnabled() }
func GetIPCHost() string                                     { return cfg.GetIPCHost() }
func GetDevTrackServerHTTPPort() int                         { return cfg.GetDevTrackServerHTTPPort() }
func GetHTTPTimeoutShort() int                               { return cfg.GetHTTPTimeoutShort() }

// ── Function forwards — server_config.go ─────────────────────────────────────

func GetServerMode() ServerMode                              { return cfg.GetServerMode() }
func GetServerURL() string                                   { return cfg.GetServerURL() }
func IsTLSEnabled() bool                                     { return cfg.IsTLSEnabled() }
func GetTLSCertPath() string                                 { return cfg.GetTLSCertPath() }
func GetTLSKeyPath() string                                  { return cfg.GetTLSKeyPath() }
func IsExternalServer() bool                                 { return cfg.IsExternalServer() }
func IsLocalTLS() bool                                       { return cfg.IsLocalTLS() }
func RunInstall() error                                      { return cfg.RunInstall() }

// ── Function forwards — cloud.go ─────────────────────────────────────────────

func LoadCloudConfig() (*CloudConfig, error)                 { return cfg.LoadCloudConfig() }
func SaveCloudConfig(c *CloudConfig) error                   { return cfg.SaveCloudConfig(c) }
func ClearCloudConfig() error                                { return cfg.ClearCloudConfig() }
func IsCloudMode() bool                                      { return cfg.IsCloudMode() }
func GetCloudAPIKey() string                                 { return cfg.GetCloudAPIKey() }
func GetCloudURL() string                                    { return cfg.GetCloudURL() }

// ── Function forwards — loadenv.go ───────────────────────────────────────────

func AutoLoadEnv()                                           { cfg.AutoLoadEnv() }
func RegisterEnvFile(p string) error                         { return cfg.RegisterEnvFile(p) }
func resolveEnvFilePath() string                             { return cfg.ResolveEnvFilePath() }
func SetBuildVersion(v string)                               { cfg.SetBuildVersion(v) }
func devtrackDataHome() (string, error)                      { return cfg.DevtrackDataHome() }
