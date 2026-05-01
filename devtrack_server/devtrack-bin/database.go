package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// Database represents the SQLite database connection
type Database struct {
	db   *sql.DB
	path string
}

// TriggerRecord represents a trigger event in the database
type TriggerRecord struct {
	ID            int64
	TriggerType   string
	Timestamp     time.Time
	Source        string
	RepoPath      string
	CommitHash    string
	CommitMessage string
	Author        string
	Data          string // JSON data
	Processed     bool
}

// ResponseRecord represents a user response in the database
type ResponseRecord struct {
	ID          int64
	TriggerID   int64
	Timestamp   time.Time
	Project     string
	TicketID    string
	Description string
	TimeSpent   string
	Status      string
	RawInput    string
}

// TaskUpdateRecord represents a task update in the database
type TaskUpdateRecord struct {
	ID         int64
	ResponseID int64
	Timestamp  time.Time
	Project    string
	TicketID   string
	UpdateText string
	Status     string
	Synced     bool
	SyncedAt   *time.Time
	Platform   string // "azure_devops", "github", "jira"
	Error      string
}

// LogRecord represents a log entry in the database
type LogRecord struct {
	ID        int64
	Timestamp time.Time
	Level     string
	Component string
	Message   string
	Data      string // JSON data
}

// ReportRecord represents a generated report in the database
type ReportRecord struct {
	ID             int64
	ReportDate     time.Time
	ReportType     string // "daily", "weekly"
	Format         string // "text", "html", "markdown", "json"
	Content        string // Full report content
	Summary        string // Brief summary
	TotalHours     float64
	TaskCount      int
	CompletedCount int
	ProjectsCount  int
	AIEnhanced     bool
	EmailSent      bool
	EmailSentAt    *time.Time
	CreatedAt      time.Time
}

// NewDatabase creates a new database connection
func NewDatabase() (*Database, error) {
	// Get database path
	// Database location from env config
	dbDir := GetDatabaseDir()
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	dbPath := GetDatabasePath()

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{
		db:   db,
		path: dbPath,
	}

	// Initialize schema
	if err := database.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Printf("Database initialized: %s", dbPath)
	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// initSchema creates the database tables if they don't exist
func (d *Database) initSchema() error {
	schema := `
	-- Triggers table: stores all trigger events
	CREATE TABLE IF NOT EXISTS triggers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trigger_type TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		source TEXT NOT NULL,
		repo_path TEXT,
		commit_hash TEXT,
		commit_message TEXT,
		author TEXT,
		data TEXT,
		processed BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Responses table: stores user responses to triggers
	CREATE TABLE IF NOT EXISTS responses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trigger_id INTEGER NOT NULL,
		timestamp DATETIME NOT NULL,
		project TEXT,
		ticket_id TEXT,
		description TEXT,
		time_spent TEXT,
		status TEXT,
		raw_input TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (trigger_id) REFERENCES triggers(id)
	);

	-- Task updates table: stores updates to task management systems
	CREATE TABLE IF NOT EXISTS task_updates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		response_id INTEGER,
		timestamp DATETIME NOT NULL,
		project TEXT NOT NULL,
		ticket_id TEXT NOT NULL,
		update_text TEXT,
		status TEXT,
		synced BOOLEAN DEFAULT 0,
		synced_at DATETIME,
		platform TEXT,
		error TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (response_id) REFERENCES responses(id)
	);

	-- Logs table: stores application logs
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		level TEXT NOT NULL,
		component TEXT NOT NULL,
		message TEXT NOT NULL,
		data TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Config table: stores configuration key-value pairs
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Reports table: stores generated daily/weekly reports
	CREATE TABLE IF NOT EXISTS reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		report_date DATE NOT NULL,
		report_type TEXT NOT NULL,
		format TEXT NOT NULL,
		content TEXT NOT NULL,
		summary TEXT,
		total_hours REAL DEFAULT 0,
		task_count INTEGER DEFAULT 0,
		completed_count INTEGER DEFAULT 0,
		projects_count INTEGER DEFAULT 0,
		ai_enhanced BOOLEAN DEFAULT 0,
		email_sent BOOLEAN DEFAULT 0,
		email_sent_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Create indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_triggers_timestamp ON triggers(timestamp);
	CREATE INDEX IF NOT EXISTS idx_triggers_type ON triggers(trigger_type);
	CREATE INDEX IF NOT EXISTS idx_triggers_processed ON triggers(processed);
	CREATE INDEX IF NOT EXISTS idx_responses_trigger ON responses(trigger_id);
	CREATE INDEX IF NOT EXISTS idx_responses_timestamp ON responses(timestamp);
	CREATE INDEX IF NOT EXISTS idx_task_updates_response ON task_updates(response_id);
	CREATE INDEX IF NOT EXISTS idx_task_updates_synced ON task_updates(synced);
	CREATE INDEX IF NOT EXISTS idx_task_updates_platform ON task_updates(platform);
	CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
	CREATE INDEX IF NOT EXISTS idx_logs_component ON logs(component);
	CREATE INDEX IF NOT EXISTS idx_reports_date ON reports(report_date);
	CREATE INDEX IF NOT EXISTS idx_reports_type ON reports(report_type);
	`

	_, err := d.db.Exec(schema)
	return err
}

// InsertTrigger inserts a trigger record into the database
func (d *Database) InsertTrigger(record TriggerRecord) (int64, error) {
	query := `
		INSERT INTO triggers (trigger_type, timestamp, source, repo_path, commit_hash, commit_message, author, data, processed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.Exec(query,
		record.TriggerType,
		record.Timestamp,
		record.Source,
		record.RepoPath,
		record.CommitHash,
		record.CommitMessage,
		record.Author,
		record.Data,
		record.Processed,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert trigger: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// InsertResponse inserts a response record into the database
func (d *Database) InsertResponse(record ResponseRecord) (int64, error) {
	query := `
		INSERT INTO responses (trigger_id, timestamp, project, ticket_id, description, time_spent, status, raw_input)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.Exec(query,
		record.TriggerID,
		record.Timestamp,
		record.Project,
		record.TicketID,
		record.Description,
		record.TimeSpent,
		record.Status,
		record.RawInput,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert response: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// InsertTaskUpdate inserts a task update record into the database
func (d *Database) InsertTaskUpdate(record TaskUpdateRecord) (int64, error) {
	query := `
		INSERT INTO task_updates (response_id, timestamp, project, ticket_id, update_text, status, synced, synced_at, platform, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.Exec(query,
		record.ResponseID,
		record.Timestamp,
		record.Project,
		record.TicketID,
		record.UpdateText,
		record.Status,
		record.Synced,
		record.SyncedAt,
		record.Platform,
		record.Error,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert task update: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// InsertLog inserts a log record into the database
func (d *Database) InsertLog(record LogRecord) error {
	query := `
		INSERT INTO logs (timestamp, level, component, message, data)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := d.db.Exec(query,
		record.Timestamp,
		record.Level,
		record.Component,
		record.Message,
		record.Data,
	)
	if err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	return nil
}

// GetTriggerByID retrieves a trigger by ID
func (d *Database) GetTriggerByID(id int64) (*TriggerRecord, error) {
	query := `
		SELECT id, trigger_type, timestamp, source, repo_path, commit_hash, commit_message, author, data, processed
		FROM triggers
		WHERE id = ?
	`

	var record TriggerRecord
	err := d.db.QueryRow(query, id).Scan(
		&record.ID,
		&record.TriggerType,
		&record.Timestamp,
		&record.Source,
		&record.RepoPath,
		&record.CommitHash,
		&record.CommitMessage,
		&record.Author,
		&record.Data,
		&record.Processed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get trigger: %w", err)
	}

	return &record, nil
}

// GetRecentTriggers retrieves recent triggers
func (d *Database) GetRecentTriggers(limit int) ([]TriggerRecord, error) {
	query := `
		SELECT id, trigger_type, timestamp, source, repo_path, commit_hash, commit_message, author, data, processed
		FROM triggers
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer rows.Close()

	var triggers []TriggerRecord
	for rows.Next() {
		var record TriggerRecord
		err := rows.Scan(
			&record.ID,
			&record.TriggerType,
			&record.Timestamp,
			&record.Source,
			&record.RepoPath,
			&record.CommitHash,
			&record.CommitMessage,
			&record.Author,
			&record.Data,
			&record.Processed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trigger: %w", err)
		}
		triggers = append(triggers, record)
	}

	return triggers, nil
}

// GetUnsyncedTaskUpdates retrieves task updates that haven't been synced
func (d *Database) GetUnsyncedTaskUpdates() ([]TaskUpdateRecord, error) {
	query := `
		SELECT id, response_id, timestamp, project, ticket_id, update_text, status, synced, synced_at, platform, error
		FROM task_updates
		WHERE synced = 0
		ORDER BY timestamp ASC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query task updates: %w", err)
	}
	defer rows.Close()

	var updates []TaskUpdateRecord
	for rows.Next() {
		var record TaskUpdateRecord
		err := rows.Scan(
			&record.ID,
			&record.ResponseID,
			&record.Timestamp,
			&record.Project,
			&record.TicketID,
			&record.UpdateText,
			&record.Status,
			&record.Synced,
			&record.SyncedAt,
			&record.Platform,
			&record.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task update: %w", err)
		}
		updates = append(updates, record)
	}

	return updates, nil
}

// MarkTaskUpdateSynced marks a task update as synced
func (d *Database) MarkTaskUpdateSynced(id int64) error {
	query := `
		UPDATE task_updates
		SET synced = 1, synced_at = ?
		WHERE id = ?
	`

	_, err := d.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark task update as synced: %w", err)
	}

	return nil
}

// MarkTriggerProcessed marks a trigger as processed
func (d *Database) MarkTriggerProcessed(id int64) error {
	query := `
		UPDATE triggers
		SET processed = 1
		WHERE id = ?
	`

	_, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark trigger as processed: %w", err)
	}

	return nil
}

// GetConfig retrieves a configuration value
func (d *Database) GetConfig(key string) (string, error) {
	query := `SELECT value FROM config WHERE key = ?`

	var value string
	err := d.db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("config key not found: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get config: %w", err)
	}

	return value, nil
}

// SetConfig sets a configuration value
func (d *Database) SetConfig(key, value string) error {
	query := `
		INSERT INTO config (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?
	`

	now := time.Now()
	_, err := d.db.Exec(query, key, value, now, value, now)
	if err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	return nil
}

// CleanOldRecords removes records older than the specified retention period
func (d *Database) CleanOldRecords(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Clean old logs
	_, err := d.db.Exec("DELETE FROM logs WHERE timestamp < ?", cutoff)
	if err != nil {
		return fmt.Errorf("failed to clean old logs: %w", err)
	}

	// Clean old processed triggers (keep unprocessed ones)
	_, err = d.db.Exec("DELETE FROM triggers WHERE timestamp < ? AND processed = 1", cutoff)
	if err != nil {
		return fmt.Errorf("failed to clean old triggers: %w", err)
	}

	log.Printf("Cleaned records older than %d days", retentionDays)
	return nil
}

// GetStats returns database statistics
func (d *Database) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count triggers
	var triggerCount int
	err := d.db.QueryRow("SELECT COUNT(*) FROM triggers").Scan(&triggerCount)
	if err != nil {
		return nil, err
	}
	stats["triggers"] = triggerCount

	// Count responses
	var responseCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM responses").Scan(&responseCount)
	if err != nil {
		return nil, err
	}
	stats["responses"] = responseCount

	// Count task updates
	var updateCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM task_updates").Scan(&updateCount)
	if err != nil {
		return nil, err
	}
	stats["task_updates"] = updateCount

	// Count unsynced updates
	var unsyncedCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM task_updates WHERE synced = 0").Scan(&unsyncedCount)
	if err != nil {
		return nil, err
	}
	stats["unsynced_updates"] = unsyncedCount

	// Count logs
	var logCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&logCount)
	if err != nil {
		return nil, err
	}
	stats["logs"] = logCount

	stats["database_path"] = d.path

	// Count reports
	var reportCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM reports").Scan(&reportCount)
	if err != nil {
		return nil, err
	}
	stats["reports"] = reportCount

	return stats, nil
}

// GetAnalytics returns analytics: triggers today/week, top projects
func (d *Database) GetAnalytics() (map[string]interface{}, error) {
	analytics := make(map[string]interface{})

	// Triggers today
	var today int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM triggers
		WHERE date(timestamp) = date('now')
	`).Scan(&today)
	if err == nil {
		analytics["triggers_today"] = today
	}

	// Triggers this week
	var week int
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM triggers
		WHERE timestamp >= date('now', '-7 days')
	`).Scan(&week)
	if err == nil {
		analytics["triggers_this_week"] = week
	}

	// Top projects by task update count (last 30 days)
	rows, err := d.db.Query(`
		SELECT project, COUNT(*) as cnt FROM task_updates
		WHERE timestamp >= date('now', '-30 days') AND project != ''
		GROUP BY project ORDER BY cnt DESC LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		var top []map[string]interface{}
		for rows.Next() {
			var p string
			var c int64
			if rows.Scan(&p, &c) == nil {
				top = append(top, map[string]interface{}{"project": p, "count": c})
			}
		}
		analytics["top_projects"] = top
	}

	return analytics, nil
}

// InsertReport inserts a report record into the database
func (d *Database) InsertReport(record ReportRecord) (int64, error) {
	query := `
		INSERT INTO reports (report_date, report_type, format, content, summary, total_hours, task_count, completed_count, projects_count, ai_enhanced, email_sent, email_sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.Exec(query,
		record.ReportDate,
		record.ReportType,
		record.Format,
		record.Content,
		record.Summary,
		record.TotalHours,
		record.TaskCount,
		record.CompletedCount,
		record.ProjectsCount,
		record.AIEnhanced,
		record.EmailSent,
		record.EmailSentAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert report: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// GetReportByID retrieves a report by ID
func (d *Database) GetReportByID(id int64) (*ReportRecord, error) {
	query := `
		SELECT id, report_date, report_type, format, content, summary, total_hours, task_count, completed_count, projects_count, ai_enhanced, email_sent, email_sent_at, created_at
		FROM reports
		WHERE id = ?
	`

	var record ReportRecord
	err := d.db.QueryRow(query, id).Scan(
		&record.ID,
		&record.ReportDate,
		&record.ReportType,
		&record.Format,
		&record.Content,
		&record.Summary,
		&record.TotalHours,
		&record.TaskCount,
		&record.CompletedCount,
		&record.ProjectsCount,
		&record.AIEnhanced,
		&record.EmailSent,
		&record.EmailSentAt,
		&record.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get report: %w", err)
	}

	return &record, nil
}

// GetReports retrieves reports with optional filters
func (d *Database) GetReports(reportType string, days int, limit int) ([]ReportRecord, error) {
	var query string
	var args []interface{}

	if reportType != "" {
		query = `
			SELECT id, report_date, report_type, format, content, summary, total_hours, task_count, completed_count, projects_count, ai_enhanced, email_sent, email_sent_at, created_at
			FROM reports
			WHERE report_type = ? AND report_date >= date('now', '-' || ? || ' days')
			ORDER BY report_date DESC
			LIMIT ?
		`
		args = []interface{}{reportType, days, limit}
	} else {
		query = `
			SELECT id, report_date, report_type, format, content, summary, total_hours, task_count, completed_count, projects_count, ai_enhanced, email_sent, email_sent_at, created_at
			FROM reports
			WHERE report_date >= date('now', '-' || ? || ' days')
			ORDER BY report_date DESC
			LIMIT ?
		`
		args = []interface{}{days, limit}
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query reports: %w", err)
	}
	defer rows.Close()

	var reports []ReportRecord
	for rows.Next() {
		var record ReportRecord
		err := rows.Scan(
			&record.ID,
			&record.ReportDate,
			&record.ReportType,
			&record.Format,
			&record.Content,
			&record.Summary,
			&record.TotalHours,
			&record.TaskCount,
			&record.CompletedCount,
			&record.ProjectsCount,
			&record.AIEnhanced,
			&record.EmailSent,
			&record.EmailSentAt,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan report: %w", err)
		}
		reports = append(reports, record)
	}

	return reports, nil
}

// UpdateReportEmailStatus updates the email sent status for a report
func (d *Database) UpdateReportEmailStatus(id int64, sent bool) error {
	query := `
		UPDATE reports
		SET email_sent = ?, email_sent_at = CASE WHEN ? THEN CURRENT_TIMESTAMP ELSE NULL END
		WHERE id = ?
	`

	_, err := d.db.Exec(query, sent, sent, id)
	if err != nil {
		return fmt.Errorf("failed to update report email status: %w", err)
	}

	return nil
}

// GetReportByDate retrieves a report for a specific date and type
func (d *Database) GetReportByDate(reportDate time.Time, reportType string) (*ReportRecord, error) {
	query := `
		SELECT id, report_date, report_type, format, content, summary, total_hours, task_count, completed_count, projects_count, ai_enhanced, email_sent, email_sent_at, created_at
		FROM reports
		WHERE date(report_date) = date(?) AND report_type = ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	var record ReportRecord
	err := d.db.QueryRow(query, reportDate, reportType).Scan(
		&record.ID,
		&record.ReportDate,
		&record.ReportType,
		&record.Format,
		&record.Content,
		&record.Summary,
		&record.TotalHours,
		&record.TaskCount,
		&record.CompletedCount,
		&record.ProjectsCount,
		&record.AIEnhanced,
		&record.EmailSent,
		&record.EmailSentAt,
		&record.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get report by date: %w", err)
	}

	return &record, nil
}
