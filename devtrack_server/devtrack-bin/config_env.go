package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
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

// requiredEnvVars lists all required environment variables
var requiredEnvVars = []string{
	"PROJECT_ROOT",
	"DEVTRACK_HOME",
	"IPC_HOST",
	"IPC_PORT",
	"IPC_CONNECT_TIMEOUT_SECS",
	"PYTHON_BRIDGE_SCRIPT",
	"CLI_BINARY_NAME",
	"CONFIG_FILE_NAME",
	"DATABASE_FILE_NAME",
	"PID_FILE_NAME",
	"LOG_FILE_NAME",
	"LEARNING_DIR_NAME",
	"CONFIG_DIR_NAME",
	"CLI_APP_NAME",
	"CLI_DAEMON_NAME",
	"DEVTRACK_WORKSPACE",
	"DATABASE_DIR",
	"LOG_DIR",
	"PID_DIR",
	"CONFIG_DIR_PATH",
	"LEARNING_DIR_PATH",
	"OLLAMA_HOST",
	"PROMPT_INTERVAL",
	"WORK_HOURS_ONLY",
	"WORK_START_HOUR",
	"WORK_END_HOUR",
	"TIMEZONE",
	"LOG_LEVEL",
	"AUTO_SYNC",
	"OUTPUT_TYPE",
	"DAILY_REPORT_TIME",
	"WEEKLY_REPORT_DAY",
	"SEND_ON_TRIGGER",
	"SEND_DAILY_SUMMARY",
	"EMAIL_TO_ADDRESSES",
	"EMAIL_CC_ADDRESSES",
	"EMAIL_MANAGER",
	"EMAIL_SUBJECT",
	"TEAMS_CHANNEL_ID",
	"TEAMS_CHANNEL_NAME",
	"TEAMS_CHAT_ID",
	"TEAMS_CHAT_TYPE",
	"TEAMS_WEBHOOK_URL",
	"TEAMS_MENTION_USER",
	"LEARNING_PYTHON_PATH",
	"LEARNING_SCRIPT_PATH",
	"LEARNING_DEFAULT_DAYS",
	"DEVTRACK_VERSION",
	"DEVTRACK_BUILD_DATE",
}

// LoadEnvConfig loads configuration from .env files and environment variables
// Returns error if any required variable is missing
func LoadEnvConfig() (*EnvConfig, error) {
	if envConfig != nil {
		return envConfig, nil
	}

	// Only load .env from explicit location: DEVTRACK_ENV_FILE or .env in current directory
	envPath := os.Getenv("DEVTRACK_ENV_FILE")
	if envPath == "" {
		envPath = ".env"
	}
	if !fileExists(envPath) {
		return nil, fmt.Errorf(".env file not found at: %s. Set DEVTRACK_ENV_FILE or place .env in current directory.", envPath)
	}
	if err := godotenv.Load(envPath); err != nil {
		return nil, fmt.Errorf("failed to load .env file at %s: %v", envPath, err)
	}

	// Check all required variables
	missing := []string{}
	for _, key := range requiredEnvVars {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables:\n%s", strings.Join(missing, "\n"))
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

// GetPythonBridgePath returns the path to python_bridge.py
func GetPythonBridgePath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	path := filepath.Join(config.ProjectRoot, config.PythonBridgeScript)
	if !fileExists(path) {
		fmt.Fprintf(os.Stderr, "ERROR: Python bridge script not found at: %s\n", path)
		fmt.Fprintf(os.Stderr, "Check PROJECT_ROOT and PYTHON_BRIDGE_SCRIPT in .env file\n")
		os.Exit(1)
	}

	return path
}

// GetEmailReporterPath returns the path to backend/email_reporter.py
func GetEmailReporterPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	path := filepath.Join(config.ProjectRoot, "backend", "email_reporter.py")
	if !fileExists(path) {
		fmt.Fprintf(os.Stderr, "ERROR: Email reporter script not found at: %s\n", path)
		fmt.Fprintf(os.Stderr, "Check PROJECT_ROOT in .env file\n")
		os.Exit(1)
	}

	return path
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

func GetLearningDailyScriptPath() string {
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	// Use explicit env var if set, otherwise derive from PROJECT_ROOT
	if config.LearningDailyScriptPath != "" {
		return expandPath(config.LearningDailyScriptPath)
	}
	return filepath.Join(config.ProjectRoot, "backend", "run_daily_learning.py")
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
	config, err := LoadEnvConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	return config.DevTrackVersion
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
