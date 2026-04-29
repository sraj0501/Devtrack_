package main

import (
	"fmt"
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
	LearningPythonPath      string
	LearningScriptPath      string
	LearningDailyScriptPath string
	LearningDefaultDays     string

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
		LearningPythonPath:      os.Getenv("LEARNING_PYTHON_PATH"),
		LearningScriptPath:      os.Getenv("LEARNING_SCRIPT_PATH"),
		LearningDailyScriptPath: os.Getenv("LEARNING_DAILY_SCRIPT_PATH"),
		LearningDefaultDays:     os.Getenv("LEARNING_DEFAULT_DAYS"),
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

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetDevTrackDir returns the DevTrack home directory
func GetDevTrackDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.DevTrackHome
}

// GetIPCAddress returns the full IPC address (host:port)
func GetIPCAddress() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.IPCHost + ":" + config.IPCPort
}

// GetPythonBridgePath returns the path to python_bridge.py.
// Returns an error instead of calling os.Exit so Lightweight mode callers can
// handle the missing file gracefully.
func GetPythonBridgePath() (string, error) {
	config, err := LoadEnvConfig()
	if err != nil {
		return "", fmt.Errorf("config load failed: %w", err)
	}

	path := filepath.Join(config.ProjectRoot, config.PythonBridgeScript)
	if !fileExists(path) {
		return "", fmt.Errorf("Python bridge script not found at %s", path)
	}

	return path, nil
}

// GetEmailReporterPath returns the path to backend/email_reporter.py.
// Returns an error instead of calling os.Exit so Lightweight mode callers can
// handle the missing backend gracefully.
func GetEmailReporterPath() (string, error) {
	config, err := LoadEnvConfig()
	if err != nil {
		return "", fmt.Errorf("config load failed: %w", err)
	}

	path := filepath.Join(config.ProjectRoot, "backend", "email_reporter.py")
	if !fileExists(path) {
		return "", fmt.Errorf("email reporter script not found at %s (Managed mode required)", path)
	}

	return path, nil
}

// GetConfigFileName returns the config file name
func GetConfigFileName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.ConfigFileName
}

func GetConfigDirPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.ConfigDirPath
}

// GetDatabaseFileName returns the database file name
func GetDatabaseFileName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.DatabaseFileName
}

func GetDatabaseDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.PIDFileName
}

func GetPIDDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.LogFileName
}

func GetLogDir() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.LearningDirName
}

func GetLearningDirPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.LearningDirPath
}

// GetCLIAppName returns the CLI application name
func GetCLIAppName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.CLIAppName
}

func mustParseInt(name, raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid integer value for %s: %s\n", name, raw)
		os.Exit(1)
	}
	return value
}

func mustParseBool(name, raw string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid boolean value for %s: %s\n", name, raw)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseInt("PROMPT_INTERVAL", config.PromptInterval)
}

func GetWorkHoursOnly() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseBool("WORK_HOURS_ONLY", config.WorkHoursOnly)
}

func GetWorkStartHour() int {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseInt("WORK_START_HOUR", config.WorkStartHour)
}

func GetWorkEndHour() int {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseInt("WORK_END_HOUR", config.WorkEndHour)
}

func GetTimezone() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.Timezone
}

func GetLogLevel() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.LogLevel
}

func GetAutoSync() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseBool("AUTO_SYNC", config.AutoSync)
}

func GetOutputType() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.OutputType
}

func GetDailyReportTime() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.DailyReportTime
}

func GetWeeklyReportDay() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.WeeklyReportDay
}

func GetSendOnTrigger() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseBool("SEND_ON_TRIGGER", config.SendOnTrigger)
}

func GetSendDailySummary() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseBool("SEND_DAILY_SUMMARY", config.SendDailySummary)
}

func GetEmailToAddresses() []string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return splitCSV(config.EmailToAddresses)
}

func GetEmailCCAddresses() []string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return splitCSV(config.EmailCCAddresses)
}

func GetEmailManager() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.EmailManager
}

func GetEmailSubject() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.EmailSubject
}

func GetTeamsChannelID() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.TeamsChannelID
}

func GetTeamsChannelName() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.TeamsChannelName
}

func GetTeamsChatID() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.TeamsChatID
}

func GetTeamsChatType() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.TeamsChatType
}

func GetTeamsWebhookURL() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.TeamsWebhookURL
}

func GetTeamsMentionUser() bool {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseBool("TEAMS_MENTION_USER", config.TeamsMentionUser)
}

func GetLearningPythonPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.LearningPythonPath
}

func GetLearningScriptPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return expandPath(config.LearningScriptPath)
}

// GetLearningDailyScriptPath returns the path to the daily learning Python script.
// Returns an error instead of calling os.Exit so Lightweight mode callers can
// handle the missing backend gracefully.
func GetLearningDailyScriptPath() (string, error) {
	config, err := LoadEnvConfig()
	if err != nil {
		return "", fmt.Errorf("config load failed: %w", err)
	}
	var path string
	// Use explicit env var if set, otherwise derive from PROJECT_ROOT
	if config.LearningDailyScriptPath != "" {
		path = expandPath(config.LearningDailyScriptPath)
	} else {
		path = filepath.Join(config.ProjectRoot, "backend", "run_daily_learning.py")
	}
	if !fileExists(path) {
		return "", fmt.Errorf("daily learning script not found at %s (Managed mode required)", path)
	}
	return path, nil
}

func GetLearningDefaultDays() int {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return mustParseInt("LEARNING_DEFAULT_DAYS", config.LearningDefaultDays)
}

func GetDevTrackVersion() string {
	// Prefer ldflags-injected version (set by GoReleaser at build time)
	if Version != "dev" {
		return Version
	}
	config, err := LoadEnvConfig()
	if err != nil {
		return Version // return "dev" rather than crashing
	}
	if config.DevTrackVersion != "" {
		return config.DevTrackVersion
	}
	return Version
}

func GetDevTrackBuildDate() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.DevTrackBuildDate
}

// GetIPCConnectTimeoutSecs returns the IPC connection timeout in seconds
// REQUIRED: IPC_CONNECT_TIMEOUT_SECS must be set in .env
func GetIPCConnectTimeoutSecs() int {
	val := os.Getenv("IPC_CONNECT_TIMEOUT_SECS")
	if val == "" {
		fmt.Fprintf(os.Stderr, "ERROR: IPC_CONNECT_TIMEOUT_SECS not set in .env\n")
		os.Exit(1)
	}
	secs := mustParseInt("IPC_CONNECT_TIMEOUT_SECS", val)
	if secs <= 0 {
		fmt.Fprintf(os.Stderr, "ERROR: IPC_CONNECT_TIMEOUT_SECS must be > 0, got: %d\n", secs)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(config.ProjectRoot, "workspaces.yaml")
}

// IsWebhookEnabled returns whether the webhook server is enabled.
// Reads WEBHOOK_ENABLED from .env (default: false).
func IsWebhookEnabled() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("WEBHOOK_ENABLED")))
	return val == "true" || val == "1" || val == "yes" || val == "on"
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
		fmt.Fprintf(os.Stderr, "ERROR: Invalid integer value for WEBHOOK_PORT: %s\n", val)
		os.Exit(1)
	}
	if port <= 0 || port > 65535 {
		fmt.Fprintf(os.Stderr, "ERROR: WEBHOOK_PORT must be between 1 and 65535, got: %d\n", port)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: HEALTH_CHECK_INTERVAL_SECS must be > 0, got: %d\n", secs)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: HEALTH_MAX_RESTARTS_PER_HOUR must be >= 0, got: %d\n", n)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: QUEUE_DRAIN_INTERVAL_SECS must be > 0, got: %d\n", secs)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: QUEUE_MAX_RETRIES must be >= 0, got: %d\n", n)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: QUEUE_RETENTION_DAYS must be > 0, got: %d\n", days)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "ERROR: DEFERRED_COMMIT_EXPIRY_HOURS must be > 0, got: %d\n", hours)
		os.Exit(1)
	}
	return hours
}

// IsTelegramEnabled returns whether the Telegram bot is enabled
func IsTelegramEnabled() bool {
	val := os.Getenv("TELEGRAM_ENABLED")
	return strings.EqualFold(val, "true") || val == "1"
}

// IsSlackEnabled returns whether the Slack bot is enabled
func IsSlackEnabled() bool {
	val := os.Getenv("SLACK_ENABLED")
	return strings.EqualFold(val, "true") || val == "1"
}

func IsAzurePollerEnabled() bool {
	val := os.Getenv("AZURE_POLL_ENABLED")
	return strings.EqualFold(val, "true") || val == "1"
}

// IsGitLabPollerEnabled returns whether the GitLab assignment poller is enabled
func IsGitLabPollerEnabled() bool {
	val := os.Getenv("GITLAB_POLL_ENABLED")
	return strings.EqualFold(val, "true") || val == "1"
}

// GetHealthAutoRestartTelegram returns whether to auto-restart the Telegram bot
func GetHealthAutoRestartTelegram() bool {
	val := os.Getenv("HEALTH_AUTO_RESTART_TELEGRAM")
	if val == "" {
		return true // default: auto-restart
	}
	return strings.EqualFold(val, "true") || val == "1"
}
