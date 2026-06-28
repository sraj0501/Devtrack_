package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EnvConfig holds all environment configuration
// All fields are REQUIRED - no fallbacks
type EnvConfig struct {
	// Paths
	ProjectRoot     string
	DevTrackHome    string
	Workspace       string
	DatabaseDir     string
	LogDir          string
	PIDDir          string
	ConfigDirPath   string
	LearningDirPath string

	// IPC Configuration
	IPCHost string
	IPCPort string

	// File names
	PythonBridgeScript string
	CLIBinaryName      string
	ConfigFileName     string
	DatabaseFileName   string
	PIDFileName        string
	LogFileName        string

	// Directory names
	LearningDirName string
	ConfigDirName   string

	// CLI identifiers
	CLIAppName    string
	CLIDaemonName string

	// External services
	OllamaHost string

	// App settings
	PromptInterval   string
	WorkHoursOnly    string
	WorkStartHour    string
	WorkEndHour      string
	Timezone         string
	LogLevel         string
	AutoSync         string
	OutputType       string
	DailyReportTime  string
	WeeklyReportDay  string
	SendOnTrigger    string
	SendDailySummary string

	// Notification settings
	EmailToAddresses string
	EmailCCAddresses string
	EmailManager     string
	EmailSubject     string
	TeamsChannelID   string
	TeamsChannelName string
	TeamsChatID      string
	TeamsChatType    string
	TeamsWebhookURL  string
	TeamsMentionUser string

	// Learning command settings
	LearningDefaultDays string

	// Build metadata
	DevTrackVersion   string
	DevTrackBuildDate string
}

var envConfig *EnvConfig

// requiredEnvVars lists the environment variables that must be present for the
// daemon to start. Feature-specific variables (Ollama, Teams, Email, Telegram,
// Learning, etc.) are validated at the point of use, not at startup.
var requiredEnvVars = []string{
	// Daemon identity & paths
	"PROJECT_ROOT",
	"DEVTRACK_HOME",
	"CLI_APP_NAME",
	"CLI_DAEMON_NAME",
	"CLI_BINARY_NAME",
	// IPC
	"IPC_HOST",
	"IPC_PORT",
	"IPC_CONNECT_TIMEOUT_SECS",
	// Data directories
	"DATABASE_DIR",
	"LOG_DIR",
	"PID_DIR",
	"CONFIG_DIR_PATH",
	"LEARNING_DIR_PATH",
	// File names
	"CONFIG_FILE_NAME",
	"DATABASE_FILE_NAME",
	"PID_FILE_NAME",
	"LOG_FILE_NAME",
	"LEARNING_DIR_NAME",
	"CONFIG_DIR_NAME",
	// Scheduler
	"PROMPT_INTERVAL",
	"WORK_HOURS_ONLY",
	"WORK_START_HOUR",
	"WORK_END_HOUR",
	"TIMEZONE",
	"LOG_LEVEL",
	"AUTO_SYNC",
}

// LoadEnvConfig reads configuration from the process environment.
// How those variables get into the environment (secret manager, shell export,
// launchd, Docker, etc.) is not this function's concern.
// Returns an error listing any required variables that are absent.
func LoadEnvConfig() (*EnvConfig, error) {
	if envConfig != nil {
		return envConfig, nil
	}

	// Validate required variables are present in the environment.
	missing := []string{}
	for _, key := range requiredEnvVars {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"missing required environment variables:\n  %s\n\n"+
				"First time? Run: devtrack setup\n"+
				"Already configured? Run: source /path/to/.env && devtrack start\n"+
				"See docs/CONFIGURATION.md for details.",
			strings.Join(missing, "\n  "),
		)
	}

	// Build config from environment
	config := &EnvConfig{
		ProjectRoot:         expandPath(os.Getenv("PROJECT_ROOT")),
		DevTrackHome:        expandPath(os.Getenv("DEVTRACK_HOME")),
		IPCHost:             os.Getenv("IPC_HOST"),
		IPCPort:             os.Getenv("IPC_PORT"),
		DatabaseDir:         expandPath(os.Getenv("DATABASE_DIR")),
		LogDir:              expandPath(os.Getenv("LOG_DIR")),
		PIDDir:              expandPath(os.Getenv("PID_DIR")),
		ConfigDirPath:       expandPath(os.Getenv("CONFIG_DIR_PATH")),
		LearningDirPath:     expandPath(os.Getenv("LEARNING_DIR_PATH")),
		PythonBridgeScript:  os.Getenv("PYTHON_BRIDGE_SCRIPT"),
		CLIBinaryName:       os.Getenv("CLI_BINARY_NAME"),
		ConfigFileName:      os.Getenv("CONFIG_FILE_NAME"),
		DatabaseFileName:    os.Getenv("DATABASE_FILE_NAME"),
		PIDFileName:         os.Getenv("PID_FILE_NAME"),
		LogFileName:         os.Getenv("LOG_FILE_NAME"),
		LearningDirName:     os.Getenv("LEARNING_DIR_NAME"),
		ConfigDirName:       os.Getenv("CONFIG_DIR_NAME"),
		CLIAppName:          os.Getenv("CLI_APP_NAME"),
		CLIDaemonName:       os.Getenv("CLI_DAEMON_NAME"),
		Workspace:           expandPath(os.Getenv("DEVTRACK_WORKSPACE")),
		OllamaHost:          os.Getenv("OLLAMA_HOST"),
		PromptInterval:      os.Getenv("PROMPT_INTERVAL"),
		WorkHoursOnly:       os.Getenv("WORK_HOURS_ONLY"),
		WorkStartHour:       os.Getenv("WORK_START_HOUR"),
		WorkEndHour:         os.Getenv("WORK_END_HOUR"),
		Timezone:            os.Getenv("TIMEZONE"),
		LogLevel:            os.Getenv("LOG_LEVEL"),
		AutoSync:            os.Getenv("AUTO_SYNC"),
		OutputType:          os.Getenv("OUTPUT_TYPE"),
		DailyReportTime:     os.Getenv("DAILY_REPORT_TIME"),
		WeeklyReportDay:     os.Getenv("WEEKLY_REPORT_DAY"),
		SendOnTrigger:       os.Getenv("SEND_ON_TRIGGER"),
		SendDailySummary:    os.Getenv("SEND_DAILY_SUMMARY"),
		EmailToAddresses:    os.Getenv("EMAIL_TO_ADDRESSES"),
		EmailCCAddresses:    os.Getenv("EMAIL_CC_ADDRESSES"),
		EmailManager:        os.Getenv("EMAIL_MANAGER"),
		EmailSubject:        os.Getenv("EMAIL_SUBJECT"),
		TeamsChannelID:      os.Getenv("TEAMS_CHANNEL_ID"),
		TeamsChannelName:    os.Getenv("TEAMS_CHANNEL_NAME"),
		TeamsChatID:         os.Getenv("TEAMS_CHAT_ID"),
		TeamsChatType:       os.Getenv("TEAMS_CHAT_TYPE"),
		TeamsWebhookURL:     os.Getenv("TEAMS_WEBHOOK_URL"),
		TeamsMentionUser:    os.Getenv("TEAMS_MENTION_USER"),
		LearningDefaultDays: os.Getenv("LEARNING_DEFAULT_DAYS"),
		DevTrackVersion:     os.Getenv("DEVTRACK_VERSION"),
		DevTrackBuildDate:   os.Getenv("DEVTRACK_BUILD_DATE"),
	}

	envConfig = config
	return config, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// GetDevTrackDir returns the DevTrack home directory
func GetDevTrackDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.DevTrackHome
}

// GetIPCAddress returns the full IPC address (host:port)
func GetIPCAddress() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.IPCHost + ":" + config.IPCPort
}

// GetConfigFileName returns the config file name
func GetConfigFileName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.ConfigFileName
}

func GetConfigDirPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.ConfigDirPath
}

// GetDatabaseFileName returns the database file name
func GetDatabaseFileName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.DatabaseFileName
}

func GetDatabaseDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.DatabaseDir
}

func GetDatabasePath() string {
	return filepath.Join(GetDatabaseDir(), GetDatabaseFileName())
}

// GetPIDFileName returns the PID file name
func GetPIDFileName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.PIDFileName
}

func GetPIDDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.PIDDir
}

func GetPIDFilePath() string {
	return filepath.Join(GetPIDDir(), GetPIDFileName())
}

// GetLogFileName returns the log file name
func GetLogFileName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.LogFileName
}

func GetLogDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.LogDir
}

func GetLogFilePath() string {
	return filepath.Join(GetLogDir(), GetLogFileName())
}

// GetLearningDirName returns the learning directory name
func GetLearningDirName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.LearningDirName
}

func GetLearningDirPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.LearningDirPath
}

// GetCLIAppName returns the CLI application name
func GetCLIAppName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.CLIAppName
}

func mustParseInt(name, raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		panic(fmt.Sprintf("devtrack: %s must be an integer, got %q", name, raw))
	}
	return value
}

func mustParseBool(name, raw string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		panic(fmt.Sprintf("devtrack: %s must be a boolean (true/false/1/0), got %q", name, raw))
	}
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, item := range parts {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func GetPromptInterval() int {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseInt("PROMPT_INTERVAL", config.PromptInterval)
}

func GetWorkHoursOnly() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseBool("WORK_HOURS_ONLY", config.WorkHoursOnly)
}

func GetWorkStartHour() int {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseInt("WORK_START_HOUR", config.WorkStartHour)
}

func GetWorkEndHour() int {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseInt("WORK_END_HOUR", config.WorkEndHour)
}

func GetTimezone() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.Timezone
}

func GetLogLevel() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.LogLevel
}

func GetAutoSync() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseBool("AUTO_SYNC", config.AutoSync)
}

func GetOutputType() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.OutputType
}

func GetDailyReportTime() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.DailyReportTime
}

func GetWeeklyReportDay() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.WeeklyReportDay
}

func GetSendOnTrigger() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseBool("SEND_ON_TRIGGER", config.SendOnTrigger)
}

func GetSendDailySummary() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseBool("SEND_DAILY_SUMMARY", config.SendDailySummary)
}

func GetEmailToAddresses() []string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return splitCSV(config.EmailToAddresses)
}

func GetEmailCCAddresses() []string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return splitCSV(config.EmailCCAddresses)
}

func GetEmailManager() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.EmailManager
}

func GetEmailSubject() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.EmailSubject
}

func GetTeamsChannelID() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.TeamsChannelID
}

func GetTeamsChannelName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.TeamsChannelName
}

func GetTeamsChatID() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.TeamsChatID
}

func GetTeamsChatType() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.TeamsChatType
}

func GetTeamsWebhookURL() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.TeamsWebhookURL
}

func GetTeamsMentionUser() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseBool("TEAMS_MENTION_USER", config.TeamsMentionUser)
}

func GetLearningDefaultDays() int {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return mustParseInt("LEARNING_DEFAULT_DAYS", config.LearningDefaultDays)
}

// buildVersion holds the injected ldflags version. Set by SetBuildVersion from main.
var buildVersion = "dev"

// SetBuildVersion lets the main package push the ldflags-injected version into
// this package so GetDevTrackVersion() can return it.
func SetBuildVersion(v string) {
	buildVersion = v
}

func GetDevTrackVersion() string {
	// Prefer ldflags-injected version (set by GoReleaser at build time)
	if buildVersion != "dev" {
		return buildVersion
	}
	config, err := LoadEnvConfig()
	if err != nil {
		return buildVersion // return "dev" rather than crashing
	}
	if config.DevTrackVersion != "" {
		return config.DevTrackVersion
	}
	return buildVersion
}

func GetDevTrackBuildDate() string {
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return config.DevTrackBuildDate
}

// GetIPCConnectTimeoutSecs returns the IPC connection timeout in seconds
// REQUIRED: IPC_CONNECT_TIMEOUT_SECS must be set in .env
func GetIPCConnectTimeoutSecs() int {
	val := os.Getenv("IPC_CONNECT_TIMEOUT_SECS")
	if val == "" {
		panic("devtrack: IPC_CONNECT_TIMEOUT_SECS not set — add it to .env")
	}
	secs := mustParseInt("IPC_CONNECT_TIMEOUT_SECS", val)
	if secs <= 0 {
		panic(fmt.Sprintf("devtrack: IPC_CONNECT_TIMEOUT_SECS must be > 0, got %d", secs))
	}
	return secs
}

// GetWorkspacesFilePath returns the path to workspaces.yaml.
// Reads WORKSPACES_FILE env var; defaults to $PROJECT_ROOT/workspaces.yaml.
// The file is optional — absence means single-repo mode (backward compat).
func GetWorkspacesFilePath() string {
	if val := os.Getenv("WORKSPACES_FILE"); val != "" {
		return expandPath(val)
	}
	config, err := LoadEnvConfig()
	if err != nil {
		panic(fmt.Sprintf("devtrack: config: %v", err))
	}
	return filepath.Join(config.ProjectRoot, "workspaces.yaml")
}

// IsWebhookEnabled returns whether the webhook server is enabled.
// Reads WEBHOOK_ENABLED from .env (default: false).
func IsWebhookEnabled() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("WEBHOOK_ENABLED")))
	return val == "true" || val == "1" || val == "yes" || val == "on"
}

// --- GitHub / Ticket Sync ---

// GetGitHubToken returns the GitHub PAT from GITHUB_TOKEN.
// Exits with an error if the variable is not set.
func GetGitHubToken() string {
	val := os.Getenv("GITHUB_TOKEN")
	if val == "" {
		panic("devtrack: GITHUB_TOKEN not set — add it to .env")
	}
	return val
}

// GetGitHubDefaultRepo returns the default GitHub repo (owner/repo) from GITHUB_DEFAULT_REPO.
// Exits with an error if the variable is not set.
func GetGitHubDefaultRepo() string {
	val := os.Getenv("GITHUB_DEFAULT_REPO")
	if val == "" {
		panic("devtrack: GITHUB_DEFAULT_REPO not set — add it to .env")
	}
	return val
}

// GetGitHubAssignee returns the GitHub username to filter assigned issues from GITHUB_ASSIGNEE.
// Exits with an error if the variable is not set.
func GetGitHubAssignee() string {
	val := os.Getenv("GITHUB_ASSIGNEE")
	if val == "" {
		panic("devtrack: GITHUB_ASSIGNEE not set — add it to .env")
	}
	return val
}

// GetWebhookPort returns the webhook server listen port.
// Reads WEBHOOK_PORT from .env (default: 8089).
func GetWebhookPort() int {
	val := os.Getenv("WEBHOOK_PORT")
	if val == "" {
		return 8089
	}
	port, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		panic(fmt.Sprintf("devtrack: WEBHOOK_PORT must be an integer, got %q", val))
	}
	if port <= 0 || port > 65535 {
		panic(fmt.Sprintf("devtrack: WEBHOOK_PORT must be 1–65535, got %d", port))
	}
	return port
}

// GetHealthCheckIntervalSecs returns the health check interval in seconds
func GetHealthCheckIntervalSecs() int {
	val := os.Getenv("HEALTH_CHECK_INTERVAL_SECS")
	if val == "" {
		return 30 // sensible default for health checks
	}
	secs := mustParseInt("HEALTH_CHECK_INTERVAL_SECS", val)
	if secs <= 0 {
		panic(fmt.Sprintf("devtrack: HEALTH_CHECK_INTERVAL_SECS must be > 0, got %d", secs))
	}
	return secs
}

// GetHealthAutoRestartPython returns whether to auto-restart the Python bridge on failure
func GetHealthAutoRestartPython() bool {
	val := os.Getenv("HEALTH_AUTO_RESTART_PYTHON")
	if val == "" {
		return true
	}
	return mustParseBool("HEALTH_AUTO_RESTART_PYTHON", val)
}

// GetHealthAutoRestartWebhook returns whether to auto-restart the webhook server on failure
func GetHealthAutoRestartWebhook() bool {
	val := os.Getenv("HEALTH_AUTO_RESTART_WEBHOOK")
	if val == "" {
		return true
	}
	return mustParseBool("HEALTH_AUTO_RESTART_WEBHOOK", val)
}

// GetHealthMaxRestartsPerHour returns the maximum number of auto-restarts allowed per hour
func GetHealthMaxRestartsPerHour() int {
	val := os.Getenv("HEALTH_MAX_RESTARTS_PER_HOUR")
	if val == "" {
		return 3
	}
	n := mustParseInt("HEALTH_MAX_RESTARTS_PER_HOUR", val)
	if n < 0 {
		panic(fmt.Sprintf("devtrack: HEALTH_MAX_RESTARTS_PER_HOUR must be >= 0, got %d", n))
	}
	return n
}

// GetQueueDrainIntervalSecs returns the store-and-forward queue drain interval in seconds
func GetQueueDrainIntervalSecs() int {
	val := os.Getenv("QUEUE_DRAIN_INTERVAL_SECS")
	if val == "" {
		return 10
	}
	secs := mustParseInt("QUEUE_DRAIN_INTERVAL_SECS", val)
	if secs <= 0 {
		panic(fmt.Sprintf("devtrack: QUEUE_DRAIN_INTERVAL_SECS must be > 0, got %d", secs))
	}
	return secs
}

// GetQueueMaxRetries returns the maximum number of retries for queued items
func GetQueueMaxRetries() int {
	val := os.Getenv("QUEUE_MAX_RETRIES")
	if val == "" {
		return 10
	}
	n := mustParseInt("QUEUE_MAX_RETRIES", val)
	if n < 0 {
		panic(fmt.Sprintf("devtrack: QUEUE_MAX_RETRIES must be >= 0, got %d", n))
	}
	return n
}

// GetQueueRetentionDays returns how many days to retain completed/failed queue items
func GetQueueRetentionDays() int {
	val := os.Getenv("QUEUE_RETENTION_DAYS")
	if val == "" {
		return 7
	}
	days := mustParseInt("QUEUE_RETENTION_DAYS", val)
	if days <= 0 {
		panic(fmt.Sprintf("devtrack: QUEUE_RETENTION_DAYS must be > 0, got %d", days))
	}
	return days
}

// GetDeferredCommitExpiryHours returns the expiry time for deferred commit enhancements in hours
func GetDeferredCommitExpiryHours() int {
	val := os.Getenv("DEFERRED_COMMIT_EXPIRY_HOURS")
	if val == "" {
		return 72
	}
	hours := mustParseInt("DEFERRED_COMMIT_EXPIRY_HOURS", val)
	if hours <= 0 {
		panic(fmt.Sprintf("devtrack: DEFERRED_COMMIT_EXPIRY_HOURS must be > 0, got %d", hours))
	}
	return hours
}

// IsTelegramEnabled returns whether the Telegram bot is enabled
func IsTelegramEnabled() bool {
	val := os.Getenv("TELEGRAM_ENABLED")
	return strings.EqualFold(val, "true") || val == "1"
}

// GetIPCHost returns the daemon bind host from IPC_HOST (default 127.0.0.1).
// Used by the internal HTTP server and by Windows CLI callers.
func GetIPCHost() string {
	config, err := LoadEnvConfig()
	if err != nil {
		// Pre-setup or lightweight callers — fall back to env then loopback.
		if v := os.Getenv("IPC_HOST"); v != "" {
			return v
		}
		return "127.0.0.1"
	}
	if config.IPCHost != "" {
		return config.IPCHost
	}
	return "127.0.0.1"
}

// GetTicketSyncIntervalHours returns how often the background ticket sync runs.
// Reads TICKET_SYNC_INTERVAL_HOURS; default 4.
func GetTicketSyncIntervalHours() int {
	val := os.Getenv("TICKET_SYNC_INTERVAL_HOURS")
	if val == "" {
		return 4
	}
	h, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || h <= 0 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid TICKET_SYNC_INTERVAL_HOURS %q — using default 4\n", val)
		return 4
	}
	return h
}

// GetTicketSyncOnStart returns whether the daemon syncs tickets when it starts.
// Reads TICKET_SYNC_ON_START; default true.
func GetTicketSyncOnStart() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("TICKET_SYNC_ON_START")))
	if val == "" {
		return true
	}
	return val == "true" || val == "1" || val == "yes"
}

// GetDevTrackServerHTTPPort returns the port the daemon exposes for its internal
// HTTP control server (e.g. /internal/force-trigger).
// Reads DEVTRACK_SERVER_HTTP_PORT; default 35894.
func GetDevTrackServerHTTPPort() int {
	val := os.Getenv("DEVTRACK_SERVER_HTTP_PORT")
	if val == "" {
		return 35894
	}
	port, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || port <= 0 || port > 65535 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid DEVTRACK_SERVER_HTTP_PORT %q — using default 35894\n", val)
		return 35894
	}
	return port
}

// GetHTTPTimeoutShort returns the short HTTP client timeout in seconds used for
// internal daemon calls (e.g. force-trigger).
// Reads HTTP_TIMEOUT_SHORT_SECS; default 5.
func GetHTTPTimeoutShort() int {
	val := os.Getenv("HTTP_TIMEOUT_SHORT_SECS")
	if val == "" {
		return 5
	}
	secs, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || secs <= 0 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid HTTP_TIMEOUT_SHORT_SECS %q — using default 5\n", val)
		return 5
	}
	return secs
}

// --- Alert poller config ---

// IsAlertEnabled returns true when ALERT_ENABLED=true/1.
func IsAlertEnabled() bool {
	val := os.Getenv("ALERT_ENABLED")
	return strings.EqualFold(val, "true") || val == "1"
}

// IsAlertGitHubEnabled returns true when ALERT_GITHUB_ENABLED=true/1.
// Defaults to true when unset and GITHUB_TOKEN is present.
func IsAlertGitHubEnabled() bool {
	val := os.Getenv("ALERT_GITHUB_ENABLED")
	if val == "" {
		return os.Getenv("GITHUB_TOKEN") != ""
	}
	return strings.EqualFold(val, "true") || val == "1"
}

// IsAlertAzureEnabled returns true when ALERT_AZURE_ENABLED=true/1.
// Defaults to true when unset and AZURE_DEVOPS_PAT is present.
func IsAlertAzureEnabled() bool {
	val := os.Getenv("ALERT_AZURE_ENABLED")
	if val == "" {
		return os.Getenv("AZURE_DEVOPS_PAT") != ""
	}
	return strings.EqualFold(val, "true") || val == "1"
}

// GetReviewAgent returns the configured coding agent backend.
// Valid values: "claude-code", "copilot-cli".
// Defaults to "claude-code" if REVIEW_AGENT is unset or invalid (logs a warning).
func GetReviewAgent() string {
	v := os.Getenv("REVIEW_AGENT")
	switch v {
	case "claude-code", "copilot-cli":
		return v
	default:
		if v != "" {
			log.Printf("config: unknown REVIEW_AGENT %q — defaulting to claude-code", v)
		}
		return "claude-code"
	}
}

// GetReviewAgentTimeoutSecs returns REVIEW_AGENT_TIMEOUT_SECS (required).
// Panics with a clear message if the variable is not set.
func GetReviewAgentTimeoutSecs() int {
	val := os.Getenv("REVIEW_AGENT_TIMEOUT_SECS")
	if val == "" {
		panic("devtrack: REVIEW_AGENT_TIMEOUT_SECS not set — add it to .env (recommended value: 120)")
	}
	secs := mustParseInt("REVIEW_AGENT_TIMEOUT_SECS", val)
	if secs <= 0 {
		panic(fmt.Sprintf("devtrack: REVIEW_AGENT_TIMEOUT_SECS must be > 0, got %d", secs))
	}
	return secs
}

// GetAlertPollIntervalSecs returns ALERT_POLL_INTERVAL_SECS (default 300).
func GetAlertPollIntervalSecs() int {
	val := os.Getenv("ALERT_POLL_INTERVAL_SECS")
	if val == "" {
		return 300
	}
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || n <= 0 {
		return 300
	}
	return n
}

// GetAlertUserID returns the user identifier for alert delta-state keys.
// Priority: GITHUB_USER → EMAIL → empty string.
func GetAlertUserID() string {
	if v := os.Getenv("GITHUB_USER"); v != "" {
		return v
	}
	return os.Getenv("EMAIL")
}

// IsAlertNotifyAssigned returns ALERT_NOTIFY_ASSIGNED (default true).
func IsAlertNotifyAssigned() bool {
	val := os.Getenv("ALERT_NOTIFY_ASSIGNED")
	return val == "" || strings.EqualFold(val, "true") || val == "1"
}

// IsAlertNotifyComments returns ALERT_NOTIFY_COMMENTS (default true).
func IsAlertNotifyComments() bool {
	val := os.Getenv("ALERT_NOTIFY_COMMENTS")
	return val == "" || strings.EqualFold(val, "true") || val == "1"
}

// IsAlertNotifyStatusChanges returns ALERT_NOTIFY_STATUS_CHANGES (default true).
func IsAlertNotifyStatusChanges() bool {
	val := os.Getenv("ALERT_NOTIFY_STATUS_CHANGES")
	return val == "" || strings.EqualFold(val, "true") || val == "1"
}

// IsAlertNotifyReviewRequested returns ALERT_NOTIFY_REVIEW_REQUESTED (default true).
func IsAlertNotifyReviewRequested() bool {
	val := os.Getenv("ALERT_NOTIFY_REVIEW_REQUESTED")
	return val == "" || strings.EqualFold(val, "true") || val == "1"
}

// GetTelegramBotToken returns TELEGRAM_BOT_TOKEN.
func GetTelegramBotToken() string {
	return os.Getenv("TELEGRAM_BOT_TOKEN")
}

// GetTelegramChatIDs parses TELEGRAM_CHAT_ID (comma-separated) into a slice.
func GetTelegramChatIDs() []string {
	val := os.Getenv("TELEGRAM_CHAT_ID")
	if val == "" {
		return nil
	}
	var ids []string
	for id := range strings.SplitSeq(val, ",") {
		if s := strings.TrimSpace(id); s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}

// GetTelegramAllowedChatIDs parses TELEGRAM_ALLOWED_CHAT_IDS (comma-separated) into a slice.
// These are the chat IDs authorized to send commands to the bot.
// If empty, the bot accepts commands from anyone (dev mode).
func GetTelegramAllowedChatIDs() []string {
	val := os.Getenv("TELEGRAM_ALLOWED_CHAT_IDS")
	if val == "" {
		return nil
	}
	var ids []string
	for id := range strings.SplitSeq(val, ",") {
		if s := strings.TrimSpace(id); s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}

// GetSlackWebhookURL returns SLACK_WEBHOOK_URL.
func GetSlackWebhookURL() string {
	return os.Getenv("SLACK_WEBHOOK_URL")
}

// GetQueuePollIntervalSecs returns QUEUE_POLL_INTERVAL_SECS — how often the
// queue executor polls /queue/pending for expired actions to auto-approve.
// Reads QUEUE_POLL_INTERVAL_SECS from the environment; defaults to 15 if unset.
func GetQueuePollIntervalSecs() int {
	val := os.Getenv("QUEUE_POLL_INTERVAL_SECS")
	if val == "" {
		return 15
	}
	secs, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || secs <= 0 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid QUEUE_POLL_INTERVAL_SECS %q — using default 15\n", val)
		return 15
	}
	return secs
}

// GetEODReportHour returns the hour (0–23) at which to fire the EOD report cron.
// Reads EOD_REPORT_HOUR. Returns 0 on absent or invalid value (0 = disabled).
// Not a panic var — missing or zero simply disables the EOD auto-report.
func GetEODReportHour() int {
	val := os.Getenv("EOD_REPORT_HOUR")
	if val == "" || val == "0" {
		return 0
	}
	h, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || h < 0 || h > 23 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid EOD_REPORT_HOUR %q — EOD auto-report disabled\n", val)
		return 0
	}
	return h
}

// GetEODReportMinute returns the minute (0–59) within the EOD hour at which the
// cron fires. Reads EOD_REPORT_MINUTE. Returns 0 if absent (fires on the hour).
func GetEODReportMinute() int {
	val := os.Getenv("EOD_REPORT_MINUTE")
	if val == "" {
		return 0
	}
	m, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || m < 0 || m > 59 {
		fmt.Fprintf(os.Stderr, "WARNING: invalid EOD_REPORT_MINUTE %q — using 0\n", val)
		return 0
	}
	return m
}

// GetEODReportEmail returns the email address for EOD report delivery.
// Reads EOD_REPORT_EMAIL. Returns "" if not set (disables email delivery).
func GetEODReportEmail() string {
	return strings.TrimSpace(os.Getenv("EOD_REPORT_EMAIL"))
}

// GetWorkSessionAutoStopMinutes returns the idle duration after which an active
// work session is automatically stopped. Reads WORK_SESSION_AUTO_STOP_MINUTES.
// Returns 0 if absent or invalid (0 = disabled).
func GetWorkSessionAutoStopMinutes() int {
	val := os.Getenv("WORK_SESSION_AUTO_STOP_MINUTES")
	if val == "" {
		return 0
	}
	m, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || m <= 0 {
		return 0
	}
	return m
}

// GetEODTelegramEnabled returns whether EOD reports should be delivered via Telegram.
// Reads EOD_TELEGRAM_ENABLED. Default is false (opt-in).
// No panic — a missing var is treated as false, not a fatal misconfiguration.
func GetEODTelegramEnabled() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("EOD_TELEGRAM_ENABLED")))
	return val == "true" || val == "1"
}

// GetVoiceSeedMonths returns the number of months of git history to mine for
// voice corpus seeding (Phase 5 — Tier 0). Reads VOICE_SEED_MONTHS.
// REQUIRED: panics with a clear message if the variable is not set.
func GetVoiceSeedMonths() int {
	val := os.Getenv("VOICE_SEED_MONTHS")
	if val == "" {
		panic("devtrack: VOICE_SEED_MONTHS not set — add it to .env (recommended value: 6)")
	}
	months := mustParseInt("VOICE_SEED_MONTHS", val)
	if months <= 0 {
		panic(fmt.Sprintf("devtrack: VOICE_SEED_MONTHS must be > 0, got %d", months))
	}
	return months
}

// GetVoiceSyncIntervalHours returns how often (in hours) the background voice
// sync job polls PM platforms for PR descriptions and issue comments (Phase 5 — Tier 1).
// Reads VOICE_SYNC_INTERVAL_HOURS.
// REQUIRED: panics with a clear message if the variable is not set.
func GetVoiceSyncIntervalHours() int {
	val := os.Getenv("VOICE_SYNC_INTERVAL_HOURS")
	if val == "" {
		panic("devtrack: VOICE_SYNC_INTERVAL_HOURS not set — add it to .env (recommended value: 24)")
	}
	hours := mustParseInt("VOICE_SYNC_INTERVAL_HOURS", val)
	if hours <= 0 {
		panic(fmt.Sprintf("devtrack: VOICE_SYNC_INTERVAL_HOURS must be > 0, got %d", hours))
	}
	return hours
}

// GetMCPPort returns the MCP server port from MCP_PORT.
// Returns "0" when unset — 0 means stdio-only mode (used by Claude Code integration).
func GetMCPPort() string {
	v := os.Getenv("MCP_PORT")
	if v == "" {
		return "0"
	}
	return v
}

// GetReviewPollIntervalSecs returns REVIEW_POLL_INTERVAL_SECS (required).
// Controls how often the fix loop polls for PR approval state after each fix attempt.
// Panics with a clear message if the variable is not set.
func GetReviewPollIntervalSecs() int {
	val := os.Getenv("REVIEW_POLL_INTERVAL_SECS")
	if val == "" {
		panic("devtrack: REVIEW_POLL_INTERVAL_SECS not set — add it to .env (recommended value: 30)")
	}
	secs := mustParseInt("REVIEW_POLL_INTERVAL_SECS", val)
	if secs <= 0 {
		panic(fmt.Sprintf("devtrack: REVIEW_POLL_INTERVAL_SECS must be > 0, got %d", secs))
	}
	return secs
}
