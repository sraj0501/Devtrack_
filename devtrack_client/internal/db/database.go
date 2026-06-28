package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
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
	TicketID      string // extracted ticket ID (branch/message/active-ticket); "" = unlinked
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

// QueuedMessage represents a message in the store-and-forward queue
type QueuedMessage struct {
	ID          int64
	MessageType string
	MessageID   string
	Payload     string // JSON
	Status      string // "pending", "sent", "failed", "expired"
	RetryCount  int
	MaxRetries  int
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// DeferredCommitRecord represents a commit queued for later AI enhancement
type DeferredCommitRecord struct {
	ID              int64
	OriginalMessage string
	DiffPatch       string
	Branch          string
	RepoPath        string
	FilesChanged    string // JSON array
	Status          string // "pending", "enhanced", "committed", "expired"
	EnhancedMessage string
	BaseSHA         string // HEAD at queue time (merge base for 3-way apply)
	SnapshotSHA     string // dangling commit pinning the staged snapshot (GC-safe)
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// HealthSnapshot represents a point-in-time health check result
type HealthSnapshot struct {
	ID        int64
	Service   string
	Status    string // "up", "down", "degraded", "unconfigured"
	LatencyMs int
	Details   string // JSON
	CheckedAt time.Time
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

// WorkSessionRecord represents an active or completed work session
type WorkSessionRecord struct {
	ID              int64
	StartedAt       string
	EndedAt         *string
	TicketRef       string
	RepoPath        string
	WorkspaceName   string
	Description     string
	Commits         string // JSON array of commit hashes
	DurationMinutes *int
	AdjustedMinutes *int
	AutoStopped     bool
	CreatedAt       string
}

// NotificationRecord represents a ticket alert notification
type NotificationRecord struct {
	ID        int64
	Source    string    // "github", "azure", "jira"
	EventType string    // "assigned", "comment", "status_change", "review_requested"
	TicketID  string
	Title     string
	Body      string
	URL       string
	Read      bool
	CreatedAt time.Time
}

// NewDatabase creates a new database connection
func NewDatabase() (*Database, error) {
	// Get database path
	// Database location from env config
	dbDir := cfg.GetDatabaseDir()
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	dbPath := cfg.GetDatabasePath()

	// Open database
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &Database{
		db:   database,
		path: dbPath,
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Apply migration-managed tables (inferences, corrections, skills, etc.) so the
	// database is fully functional without requiring the full env-var setup that
	// RunPendingMigrations needs. Safe to call multiple times — all statements use
	// CREATE TABLE IF NOT EXISTS.
	if err := db.applyMigrationTables(); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to apply migration tables: %w", err)
	}

	log.Printf("Database initialized: %s", dbPath)
	return db, nil
}

// DB returns the underlying *sql.DB for use by connector packages.
func (d *Database) DB() *sql.DB {
	return d.db
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
		ticket_id TEXT DEFAULT '',
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

	-- Message queue table: store-and-forward for offline resilience
	CREATE TABLE IF NOT EXISTS message_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_type TEXT NOT NULL,
		message_id TEXT NOT NULL,
		payload TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		retry_count INTEGER DEFAULT 0,
		max_retries INTEGER DEFAULT 10,
		last_error TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Deferred commits table: commits queued for later AI enhancement
	CREATE TABLE IF NOT EXISTS deferred_commits (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_message TEXT NOT NULL,
		diff_patch TEXT,
		branch TEXT,
		repo_path TEXT,
		files_changed TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		enhanced_message TEXT,
		base_sha TEXT,
		snapshot_sha TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Health snapshots table: point-in-time health check results
	CREATE TABLE IF NOT EXISTS health_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service TEXT NOT NULL,
		status TEXT NOT NULL,
		latency_ms INTEGER DEFAULT 0,
		details TEXT,
		checked_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
	CREATE INDEX IF NOT EXISTS idx_message_queue_status ON message_queue(status);
	CREATE INDEX IF NOT EXISTS idx_deferred_commits_status ON deferred_commits(status);
	CREATE INDEX IF NOT EXISTS idx_health_snapshots_service ON health_snapshots(service);
	CREATE INDEX IF NOT EXISTS idx_health_snapshots_checked ON health_snapshots(checked_at);

	-- Work sessions table: tracks active and completed work sessions for EOD reporting
	CREATE TABLE IF NOT EXISTS work_sessions (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at       TEXT NOT NULL,
		ended_at         TEXT,
		ticket_ref       TEXT,
		repo_path        TEXT,
		workspace_name   TEXT,
		description      TEXT,
		commits          TEXT DEFAULT '[]',
		duration_minutes INTEGER,
		adjusted_minutes INTEGER,
		auto_stopped     INTEGER DEFAULT 0,
		created_at       TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_work_sessions_started ON work_sessions(started_at);
	CREATE INDEX IF NOT EXISTS idx_work_sessions_ended ON work_sessions(ended_at);

	CREATE TABLE IF NOT EXISTS vacation_mode (
		id                   INTEGER PRIMARY KEY CHECK (id = 1),
		enabled              INTEGER NOT NULL DEFAULT 0,
		enabled_at           TEXT,
		until                TEXT,
		confidence_threshold REAL    NOT NULL DEFAULT 0.7,
		auto_submit          INTEGER NOT NULL DEFAULT 1
	);
	INSERT OR IGNORE INTO vacation_mode (id, enabled) VALUES (1, 0);

	-- Notifications table: ticket alert notifications (dual-written by Python alert_poller)
	CREATE TABLE IF NOT EXISTS notifications (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		source     TEXT NOT NULL,
		event_type TEXT NOT NULL,
		ticket_id  TEXT NOT NULL,
		title      TEXT NOT NULL,
		body       TEXT DEFAULT '',
		url        TEXT DEFAULT '',
		read       INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_notifications_read    ON notifications(read);
	CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at);

	-- Alert state table: delta tracking for alert pollers
	CREATE TABLE IF NOT EXISTS alert_state (
		id           TEXT PRIMARY KEY,  -- "<user_id>:<source>"
		user_id      TEXT NOT NULL,
		source       TEXT NOT NULL,
		last_checked DATETIME NOT NULL,
		updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Ticket cache: offline-first ticket store for commit-time ranking
	CREATE TABLE IF NOT EXISTS ticket_cache (
		id          TEXT PRIMARY KEY,
		source      TEXT NOT NULL,
		external_id TEXT NOT NULL,
		repo        TEXT,
		title       TEXT NOT NULL,
		description TEXT,
		status      TEXT,
		assignee    TEXT,
		labels      TEXT,
		url         TEXT,
		synced_at   DATETIME NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- PM update queue: offline-tolerant outbox for upstream PM API calls
	CREATE TABLE IF NOT EXISTS pm_update_queue (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ticket_id   TEXT NOT NULL,
		action      TEXT NOT NULL,
		payload     TEXT NOT NULL,
		commit_hash TEXT,
		status      TEXT DEFAULT 'pending',
		attempts    INTEGER DEFAULT 0,
		last_error  TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		sent_at     DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_ticket_cache_source   ON ticket_cache(source);
	CREATE INDEX IF NOT EXISTS idx_ticket_cache_assignee ON ticket_cache(assignee);
	CREATE INDEX IF NOT EXISTS idx_ticket_cache_status   ON ticket_cache(status);
	CREATE INDEX IF NOT EXISTS idx_pm_queue_status       ON pm_update_queue(status);
	CREATE INDEX IF NOT EXISTS idx_pm_queue_ticket       ON pm_update_queue(ticket_id);
	`

	if _, err := d.db.Exec(schema); err != nil {
		return err
	}

	// Additive column migrations for pre-existing databases (CREATE TABLE
	// IF NOT EXISTS won't add columns to a table that already exists). Each
	// ALTER errors harmlessly with "duplicate column name" once applied.
	for _, alter := range []string{
		`ALTER TABLE deferred_commits ADD COLUMN base_sha TEXT`,
		`ALTER TABLE deferred_commits ADD COLUMN snapshot_sha TEXT`,
		`ALTER TABLE triggers ADD COLUMN ticket_id TEXT DEFAULT ''`,
	} {
		if _, err := d.db.Exec(alter); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migration failed (%s): %w", alter, err)
		}
	}
	return nil
}

// InsertTrigger inserts a trigger record into the database
func (d *Database) InsertTrigger(record TriggerRecord) (int64, error) {
	query := `
		INSERT INTO triggers (trigger_type, timestamp, source, repo_path, commit_hash, commit_message, author, data, processed, ticket_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		record.TicketID,
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
		SELECT id, trigger_type, timestamp, source, repo_path, commit_hash, commit_message, author, data, processed, COALESCE(ticket_id,'')
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
		&record.TicketID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get trigger: %w", err)
	}

	return &record, nil
}

// GetRecentTriggers retrieves recent triggers
func (d *Database) GetRecentTriggers(limit int) ([]TriggerRecord, error) {
	query := `
		SELECT id, trigger_type, timestamp, source, repo_path, commit_hash, commit_message, author, data, processed, COALESCE(ticket_id,'')
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
			&record.TicketID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trigger: %w", err)
		}
		triggers = append(triggers, record)
	}

	return triggers, nil
}

// GetLastTicketID returns the most recently extracted (non-empty) ticket ID
// for commit triggers in the given repo. Used by the active-ticket fallback
// strategy (TASK-069) when branch and commit-message extraction both fail.
// Returns "" with no error when no prior matched commit exists.
func (d *Database) GetLastTicketID(repoPath string) (string, error) {
	var ticketID string
	err := d.db.QueryRow(`
		SELECT ticket_id FROM triggers
		WHERE trigger_type='commit'
		  AND repo_path=?
		  AND ticket_id != ''
		  AND ticket_id != 'unlinked'
		ORDER BY timestamp DESC LIMIT 1
	`, repoPath).Scan(&ticketID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get last ticket id: %w", err)
	}
	return ticketID, nil
}

// CountTicketCommits returns the number of prior commit trigger rows that
// reference the given ticketID in the given repoPath. The caller must invoke
// this BEFORE InsertTrigger for the current commit so that only prior rows are
// counted — the check answers "have we seen this ticket in this repo before?".
// Returns (0, nil) when the ticket has never been seen. Used by the Go trigger
// flow (TASK-073) to populate CommitTriggerData.IsFirstCommitForTicket.
func (d *Database) CountTicketCommits(repoPath, ticketID string) (int, error) {
	if ticketID == "" {
		return 0, nil
	}
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM triggers
		WHERE trigger_type='commit'
		  AND repo_path=?
		  AND ticket_id=?
	`, repoPath, ticketID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountTicketCommits failed: %w", err)
	}
	return count, nil
}

// TicketStats returns ticket extraction statistics (total/linked/unlinked) for
// the last N commit triggers, optionally filtered by repo path. Pass repoPath=""
// to aggregate across all workspaces. Used by `devtrack status` (TASK-070) to
// verify the Phase 2 exit criterion: >=80% of commits mapped to a ticket.
func (d *Database) TicketStats(repoPath string, lastN int) (total, linked, unlinked int, err error) {
	row := d.db.QueryRow(`
		SELECT COUNT(*) AS total,
		       SUM(CASE WHEN ticket_id != '' AND ticket_id != 'unlinked' THEN 1 ELSE 0 END) AS linked
		FROM (
			SELECT ticket_id FROM triggers
			WHERE trigger_type='commit'
			  AND (? = '' OR repo_path = ?)
			ORDER BY timestamp DESC LIMIT ?
		)
	`, repoPath, repoPath, lastN)

	var linkedNullable sql.NullInt64
	if scanErr := row.Scan(&total, &linkedNullable); scanErr != nil {
		return 0, 0, 0, fmt.Errorf("failed to get ticket stats: %w", scanErr)
	}
	linked = int(linkedNullable.Int64)
	unlinked = total - linked
	return total, linked, unlinked, nil
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
func (d *Database) GetStats() (map[string]any, error) {
	stats := make(map[string]any)

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
func (d *Database) GetAnalytics() (map[string]any, error) {
	analytics := make(map[string]any)

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
		var top []map[string]any
		for rows.Next() {
			var p string
			var c int64
			if rows.Scan(&p, &c) == nil {
				top = append(top, map[string]any{"project": p, "count": c})
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
	var args []any

	if reportType != "" {
		query = `
			SELECT id, report_date, report_type, format, content, summary, total_hours, task_count, completed_count, projects_count, ai_enhanced, email_sent, email_sent_at, created_at
			FROM reports
			WHERE report_type = ? AND report_date >= date('now', '-' || ? || ' days')
			ORDER BY report_date DESC
			LIMIT ?
		`
		args = []any{reportType, days, limit}
	} else {
		query = `
			SELECT id, report_date, report_type, format, content, summary, total_hours, task_count, completed_count, projects_count, ai_enhanced, email_sent, email_sent_at, created_at
			FROM reports
			WHERE report_date >= date('now', '-' || ? || ' days')
			ORDER BY report_date DESC
			LIMIT ?
		`
		args = []any{days, limit}
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

// --- Message Queue CRUD ---

// EnqueueMessage inserts a message into the store-and-forward queue
func (d *Database) EnqueueMessage(msg QueuedMessage) (int64, error) {
	query := `
		INSERT INTO message_queue (message_type, message_id, payload, status, retry_count, max_retries, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := d.db.Exec(query,
		msg.MessageType,
		msg.MessageID,
		msg.Payload,
		"pending",
		0,
		msg.MaxRetries,
		"",
		now,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to enqueue message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// GetPendingMessages retrieves pending messages from the queue
func (d *Database) GetPendingMessages(limit int) ([]QueuedMessage, error) {
	query := `
		SELECT id, message_type, message_id, payload, status, retry_count, max_retries, last_error, created_at, updated_at
		FROM message_queue
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending messages: %w", err)
	}
	defer rows.Close()

	var messages []QueuedMessage
	for rows.Next() {
		var msg QueuedMessage
		err := rows.Scan(
			&msg.ID,
			&msg.MessageType,
			&msg.MessageID,
			&msg.Payload,
			&msg.Status,
			&msg.RetryCount,
			&msg.MaxRetries,
			&msg.LastError,
			&msg.CreatedAt,
			&msg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan queued message: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// MarkMessageSent marks a queued message as sent
func (d *Database) MarkMessageSent(id int64) error {
	query := `
		UPDATE message_queue
		SET status = 'sent', updated_at = ?
		WHERE id = ?
	`

	_, err := d.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark message as sent: %w", err)
	}

	return nil
}

// MarkMessageFailed marks a queued message as failed and increments retry count
func (d *Database) MarkMessageFailed(id int64, errMsg string) error {
	query := `
		UPDATE message_queue
		SET status = CASE WHEN retry_count + 1 >= max_retries THEN 'failed' ELSE 'pending' END,
			retry_count = retry_count + 1,
			last_error = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err := d.db.Exec(query, errMsg, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark message as failed: %w", err)
	}

	return nil
}

// RequeueFailedMessages requeues failed messages that haven't exhausted retries
func (d *Database) RequeueFailedMessages() (int, error) {
	query := `
		UPDATE message_queue
		SET status = 'pending', updated_at = ?
		WHERE status = 'failed' AND retry_count < max_retries
	`

	result, err := d.db.Exec(query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to requeue failed messages: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(count), nil
}

// GetMessageQueueStats returns counts of messages by status
func (d *Database) GetMessageQueueStats() (pending int, failed int, sent int, err error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END), 0)
		FROM message_queue
	`

	err = d.db.QueryRow(query).Scan(&pending, &failed, &sent)
	if err != nil {
		err = fmt.Errorf("failed to get message queue stats: %w", err)
	}

	return
}

// CleanOldMessages deletes sent messages older than the retention period
func (d *Database) CleanOldMessages(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	_, err := d.db.Exec("DELETE FROM message_queue WHERE status = 'sent' AND created_at < ?", cutoff)
	if err != nil {
		return fmt.Errorf("failed to clean old messages: %w", err)
	}

	return nil
}

// --- Deferred Commits CRUD ---

// InsertDeferredCommit inserts a deferred commit record
func (d *Database) InsertDeferredCommit(record DeferredCommitRecord) (int64, error) {
	query := `
		INSERT INTO deferred_commits (original_message, diff_patch, branch, repo_path, files_changed, status, enhanced_message, base_sha, snapshot_sha, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := d.db.Exec(query,
		record.OriginalMessage,
		record.DiffPatch,
		record.Branch,
		record.RepoPath,
		record.FilesChanged,
		"pending",
		"",
		record.BaseSHA,
		record.SnapshotSHA,
		now,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert deferred commit: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// GetPendingDeferredCommits retrieves deferred commits awaiting enhancement
func (d *Database) GetPendingDeferredCommits() ([]DeferredCommitRecord, error) {
	query := `
		SELECT id, original_message, diff_patch, branch, repo_path, files_changed, status, enhanced_message, COALESCE(base_sha,''), COALESCE(snapshot_sha,''), created_at, updated_at
		FROM deferred_commits
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending deferred commits: %w", err)
	}
	defer rows.Close()

	var records []DeferredCommitRecord
	for rows.Next() {
		var record DeferredCommitRecord
		err := rows.Scan(
			&record.ID,
			&record.OriginalMessage,
			&record.DiffPatch,
			&record.Branch,
			&record.RepoPath,
			&record.FilesChanged,
			&record.Status,
			&record.EnhancedMessage,
			&record.BaseSHA,
			&record.SnapshotSHA,
			&record.CreatedAt,
			&record.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deferred commit: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// GetEnhancedDeferredCommits retrieves deferred commits that have been enhanced
func (d *Database) GetEnhancedDeferredCommits() ([]DeferredCommitRecord, error) {
	query := `
		SELECT id, original_message, diff_patch, branch, repo_path, files_changed, status, enhanced_message, COALESCE(base_sha,''), COALESCE(snapshot_sha,''), created_at, updated_at
		FROM deferred_commits
		WHERE status = 'enhanced'
		ORDER BY created_at ASC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query enhanced deferred commits: %w", err)
	}
	defer rows.Close()

	var records []DeferredCommitRecord
	for rows.Next() {
		var record DeferredCommitRecord
		err := rows.Scan(
			&record.ID,
			&record.OriginalMessage,
			&record.DiffPatch,
			&record.Branch,
			&record.RepoPath,
			&record.FilesChanged,
			&record.Status,
			&record.EnhancedMessage,
			&record.BaseSHA,
			&record.SnapshotSHA,
			&record.CreatedAt,
			&record.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deferred commit: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// GetExpirableDeferredCommits returns the id, repo_path and snapshot_sha of
// pending/enhanced deferred commits created before cutoff. Used to prune their
// pinned snapshot refs before marking them expired.
func (d *Database) GetExpirableDeferredCommits(cutoff time.Time) ([]DeferredCommitRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, repo_path, COALESCE(snapshot_sha,'')
		FROM deferred_commits
		WHERE status IN ('pending','enhanced') AND created_at < ?
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query expirable deferred commits: %w", err)
	}
	defer rows.Close()

	var out []DeferredCommitRecord
	for rows.Next() {
		var r DeferredCommitRecord
		if err := rows.Scan(&r.ID, &r.RepoPath, &r.SnapshotSHA); err != nil {
			return nil, fmt.Errorf("failed to scan expirable deferred commit: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkDeferredCommitEnhanced marks a deferred commit as enhanced with the new message
func (d *Database) MarkDeferredCommitEnhanced(id int64, enhancedMsg string) error {
	query := `
		UPDATE deferred_commits
		SET status = 'enhanced', enhanced_message = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := d.db.Exec(query, enhancedMsg, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark deferred commit as enhanced: %w", err)
	}

	return nil
}

// MarkDeferredCommitCommitted marks a deferred commit as committed
func (d *Database) MarkDeferredCommitCommitted(id int64) error {
	query := `
		UPDATE deferred_commits
		SET status = 'committed', updated_at = ?
		WHERE id = ?
	`

	_, err := d.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark deferred commit as committed: %w", err)
	}

	return nil
}

// MarkDeferredCommitExpired marks a deferred commit as expired
func (d *Database) MarkDeferredCommitExpired(id int64) error {
	query := `
		UPDATE deferred_commits
		SET status = 'expired', updated_at = ?
		WHERE id = ?
	`

	_, err := d.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark deferred commit as expired: %w", err)
	}

	return nil
}

// GetDeferredCommitByID retrieves a deferred commit by ID
func (d *Database) GetDeferredCommitByID(id int64) (*DeferredCommitRecord, error) {
	query := `
		SELECT id, original_message, diff_patch, branch, repo_path, files_changed, status, enhanced_message, created_at, updated_at
		FROM deferred_commits
		WHERE id = ?
	`

	var record DeferredCommitRecord
	err := d.db.QueryRow(query, id).Scan(
		&record.ID,
		&record.OriginalMessage,
		&record.DiffPatch,
		&record.Branch,
		&record.RepoPath,
		&record.FilesChanged,
		&record.Status,
		&record.EnhancedMessage,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get deferred commit: %w", err)
	}

	return &record, nil
}

// GetDeferredCommitStats returns counts of deferred commits by status
func (d *Database) GetDeferredCommitStats() (pending int, enhanced int, committed int, expired int, err error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'enhanced' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'committed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'expired' THEN 1 ELSE 0 END), 0)
		FROM deferred_commits
	`

	err = d.db.QueryRow(query).Scan(&pending, &enhanced, &committed, &expired)
	if err != nil {
		err = fmt.Errorf("failed to get deferred commit stats: %w", err)
	}

	return
}

// --- Health Snapshots CRUD ---

// InsertHealthSnapshot inserts a health check snapshot
func (d *Database) InsertHealthSnapshot(snap HealthSnapshot) error {
	query := `
		INSERT INTO health_snapshots (service, status, latency_ms, details, checked_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := d.db.Exec(query,
		snap.Service,
		snap.Status,
		snap.LatencyMs,
		snap.Details,
		snap.CheckedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert health snapshot: %w", err)
	}

	return nil
}

// GetLatestHealthSnapshots retrieves the latest health snapshot per service,
// excluding any snapshot older than 10 minutes so stale entries from removed
// services (e.g. the legacy Python-era Redis/MongoDB checks) never appear.
func (d *Database) GetLatestHealthSnapshots() ([]HealthSnapshot, error) {
	threshold := time.Now().Add(-10 * time.Minute)
	query := `
		SELECT h.id, h.service, h.status, h.latency_ms, h.details, h.checked_at
		FROM health_snapshots h
		INNER JOIN (
			SELECT service, MAX(checked_at) AS max_checked
			FROM health_snapshots
			WHERE checked_at > ?
			GROUP BY service
		) latest ON h.service = latest.service AND h.checked_at = latest.max_checked
		ORDER BY h.service ASC
	`

	rows, err := d.db.Query(query, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest health snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []HealthSnapshot
	for rows.Next() {
		var snap HealthSnapshot
		err := rows.Scan(
			&snap.ID,
			&snap.Service,
			&snap.Status,
			&snap.LatencyMs,
			&snap.Details,
			&snap.CheckedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan health snapshot: %w", err)
		}
		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// CleanOldHealthSnapshots deletes health snapshots older than the retention period
func (d *Database) CleanOldHealthSnapshots(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	_, err := d.db.Exec("DELETE FROM health_snapshots WHERE checked_at < ?", cutoff)
	if err != nil {
		return fmt.Errorf("failed to clean old health snapshots: %w", err)
	}

	return nil
}

// InsertWorkSession starts a new work session and returns its ID
func (d *Database) InsertWorkSession(ticketRef, repoPath, workspaceName string) (int64, error) {
	query := `
		INSERT INTO work_sessions (started_at, ticket_ref, repo_path, workspace_name, commits)
		VALUES (datetime('now'), ?, ?, ?, '[]')
	`
	result, err := d.db.Exec(query, ticketRef, repoPath, workspaceName)
	if err != nil {
		return 0, fmt.Errorf("failed to insert work session: %w", err)
	}
	return result.LastInsertId()
}

// EndWorkSession marks a session as ended and stores the auto-measured duration
func (d *Database) EndWorkSession(id int64, endedAt string, durationMins int) error {
	query := `
		UPDATE work_sessions
		SET ended_at = ?, duration_minutes = ?
		WHERE id = ?
	`
	_, err := d.db.Exec(query, endedAt, durationMins, id)
	if err != nil {
		return fmt.Errorf("failed to end work session %d: %w", id, err)
	}
	return nil
}

// AdjustWorkSessionTime sets the user-overridden time for a session.
// The original auto-measured duration_minutes is preserved for audit purposes.
func (d *Database) AdjustWorkSessionTime(id int64, adjustedMins int) error {
	query := `UPDATE work_sessions SET adjusted_minutes = ? WHERE id = ?`
	_, err := d.db.Exec(query, adjustedMins, id)
	if err != nil {
		return fmt.Errorf("failed to adjust work session %d: %w", id, err)
	}
	return nil
}

// GetActiveWorkSession returns the first session where ended_at IS NULL, or nil if none
func (d *Database) GetActiveWorkSession() (*WorkSessionRecord, error) {
	query := `
		SELECT id, started_at, ended_at, ticket_ref, repo_path, workspace_name,
		       description, commits, duration_minutes, adjusted_minutes, auto_stopped, created_at
		FROM work_sessions
		WHERE ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`
	row := d.db.QueryRow(query)
	return scanWorkSession(row)
}

// GetWorkSessionsForDate returns all sessions that started on the given date (YYYY-MM-DD)
func (d *Database) GetWorkSessionsForDate(date string) ([]WorkSessionRecord, error) {
	query := `
		SELECT id, started_at, ended_at, ticket_ref, repo_path, workspace_name,
		       description, commits, duration_minutes, adjusted_minutes, auto_stopped, created_at
		FROM work_sessions
		WHERE date(started_at) = ?
		ORDER BY started_at ASC
	`
	rows, err := d.db.Query(query, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query work sessions for %s: %w", date, err)
	}
	defer rows.Close()

	var sessions []WorkSessionRecord
	for rows.Next() {
		s, err := scanWorkSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, rows.Err()
}

// AppendCommitToSession adds a commit hash to the JSON commits array of a session
func (d *Database) AppendCommitToSession(sessionID int64, commitHash string) error {
	// Read current commits array
	var commitsJSON string
	err := d.db.QueryRow("SELECT commits FROM work_sessions WHERE id = ?", sessionID).Scan(&commitsJSON)
	if err != nil {
		return fmt.Errorf("failed to read commits for session %d: %w", sessionID, err)
	}

	// Parse, append, re-serialize
	var commits []string
	if commitsJSON != "" && commitsJSON != "[]" {
		// Simple JSON array unmarshal via encoding/json is unavailable without import;
		// we do a lightweight string manipulation instead to avoid import churn.
		// Strip leading [ and trailing ], split on comma-quote boundaries.
		inner := commitsJSON[1 : len(commitsJSON)-1]
		if inner != "" {
			for _, part := range splitJSONStringArray(inner) {
				commits = append(commits, part)
			}
		}
	}
	commits = append(commits, commitHash)

	newJSON := buildJSONStringArray(commits)
	_, err = d.db.Exec("UPDATE work_sessions SET commits = ? WHERE id = ?", newJSON, sessionID)
	if err != nil {
		return fmt.Errorf("failed to append commit to session %d: %w", sessionID, err)
	}
	return nil
}

// scanWorkSession scans a *sql.Row into a WorkSessionRecord
func scanWorkSession(row *sql.Row) (*WorkSessionRecord, error) {
	var s WorkSessionRecord
	var autoStopped int
	err := row.Scan(
		&s.ID, &s.StartedAt, &s.EndedAt, &s.TicketRef, &s.RepoPath,
		&s.WorkspaceName, &s.Description, &s.Commits,
		&s.DurationMinutes, &s.AdjustedMinutes, &autoStopped, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan work session: %w", err)
	}
	s.AutoStopped = autoStopped == 1
	return &s, nil
}

// scanWorkSessionRow scans a *sql.Rows row into a WorkSessionRecord
func scanWorkSessionRow(rows *sql.Rows) (*WorkSessionRecord, error) {
	var s WorkSessionRecord
	var autoStopped int
	err := rows.Scan(
		&s.ID, &s.StartedAt, &s.EndedAt, &s.TicketRef, &s.RepoPath,
		&s.WorkspaceName, &s.Description, &s.Commits,
		&s.DurationMinutes, &s.AdjustedMinutes, &autoStopped, &s.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan work session row: %w", err)
	}
	s.AutoStopped = autoStopped == 1
	return &s, nil
}

// TriggerStats holds the trigger throughput snapshot served by GET /internal/stats.
// Fields match what Python's stats_client.py expects.
type TriggerStats struct {
	TriggersToday int    `json:"triggers_today"`
	CommitsToday  int    `json:"commits_today"`
	LastTrigger   string `json:"last_trigger"` // "HH:MM" local time, or "—"
	Errors24h     int    `json:"errors_24h"`
}

// GetTriggerStats returns a snapshot of today's trigger activity.
// All queries degrade gracefully: missing table → zero values, no error.
func (d *Database) GetTriggerStats() TriggerStats {
	stats := TriggerStats{LastTrigger: "—"}

	// triggers today (all types)
	_ = d.db.QueryRow(
		"SELECT COUNT(*) FROM triggers WHERE date(timestamp) = date('now')",
	).Scan(&stats.TriggersToday)

	// commit triggers today
	_ = d.db.QueryRow(
		"SELECT COUNT(*) FROM triggers WHERE trigger_type='commit' AND date(timestamp)=date('now')",
	).Scan(&stats.CommitsToday)

	// last trigger timestamp → HH:MM
	var lastTS string
	if d.db.QueryRow("SELECT timestamp FROM triggers ORDER BY timestamp DESC LIMIT 1").Scan(&lastTS) == nil && lastTS != "" {
		for _, layout := range []string{"2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, lastTS); err == nil {
				stats.LastTrigger = t.Local().Format("15:04")
				break
			}
		}
	}

	// unprocessed triggers older than 5 min and within last 24 h → errors
	_ = d.db.QueryRow(`
		SELECT COUNT(*) FROM triggers
		WHERE processed = 0
		  AND timestamp >= datetime('now','-24 hours')
		  AND timestamp <= datetime('now','-5 minutes')
	`).Scan(&stats.Errors24h)

	return stats
}

// TicketSourceSummary holds cached ticket counts per PM source (github/azure/gitlab).
type TicketSourceSummary struct {
	Source   string
	Count    int
	LastSync time.Time
}

// GetTicketCacheSummary returns row counts and latest sync timestamps for each PM
// connector table.  Tables absent from the database (pre-first-sync) are skipped.
func (d *Database) GetTicketCacheSummary() []TicketSourceSummary {
	type q struct{ source, sql string }
	queries := []q{
		{"github", "SELECT COUNT(*), COALESCE(MAX(synced_at),'') FROM github_issues"},
		{"azure", "SELECT COUNT(*), COALESCE(MAX(synced_at),'') FROM azure_workitems"},
		{"gitlab", "SELECT COUNT(*), COALESCE(MAX(synced_at),'') FROM gitlab_issues"},
	}
	var out []TicketSourceSummary
	for _, qr := range queries {
		var count int
		var lastSync string
		if err := d.db.QueryRow(qr.sql).Scan(&count, &lastSync); err != nil || count == 0 {
			continue
		}
		s := TicketSourceSummary{Source: qr.source, Count: count}
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, lastSync); err == nil {
				s.LastSync = t
				break
			}
		}
		out = append(out, s)
	}
	return out
}

// splitJSONStringArray splits the inner content of a JSON string array
// e.g. `"abc","def"` → ["abc", "def"]
func splitJSONStringArray(inner string) []string {
	var result []string
	i := 0
	for i < len(inner) {
		if inner[i] == '"' {
			j := i + 1
			for j < len(inner) && inner[j] != '"' {
				if inner[j] == '\\' {
					j++
				}
				j++
			}
			if j < len(inner) {
				result = append(result, inner[i+1:j])
				i = j + 1
			} else {
				break
			}
		} else {
			i++
		}
	}
	return result
}

// buildJSONStringArray serializes a []string as a JSON array of quoted strings
func buildJSONStringArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for idx, item := range items {
		b.WriteByte('"')
		b.WriteString(item)
		b.WriteByte('"')
		if idx < len(items)-1 {
			b.WriteByte(',')
		}
	}
	b.WriteByte(']')
	return b.String()
}

// VacationState holds the current vacation mode configuration.
type VacationState struct {
	Enabled             bool
	EnabledAt           string
	Until               string // empty = indefinite
	ConfidenceThreshold float64
	AutoSubmit          bool
}

// GetVacationState returns the current vacation mode state.
func (d *Database) GetVacationState() (*VacationState, error) {
	row := d.db.QueryRow(`SELECT enabled, enabled_at, until, confidence_threshold, auto_submit FROM vacation_mode WHERE id = 1`)
	var (
		enabled    int
		enabledAt  string
		until      string
		threshold  float64
		autoSubmit int
	)
	if err := row.Scan(&enabled, &enabledAt, &until, &threshold, &autoSubmit); err != nil {
		return nil, err
	}
	return &VacationState{
		Enabled:             enabled == 1,
		EnabledAt:           enabledAt,
		Until:               until,
		ConfidenceThreshold: threshold,
		AutoSubmit:          autoSubmit == 1,
	}, nil
}

// SetVacationMode enables or disables vacation mode.
func (d *Database) SetVacationMode(enabled bool, until string, threshold float64, autoSubmit bool) error {
	enabledAt := ""
	if enabled {
		enabledAt = time.Now().UTC().Format(time.RFC3339)
	}
	autoSubmitInt := 0
	if autoSubmit {
		autoSubmitInt = 1
	}
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := d.db.Exec(`
		UPDATE vacation_mode
		SET enabled=?, enabled_at=?, until=?, confidence_threshold=?, auto_submit=?
		WHERE id=1`,
		enabledInt, enabledAt, until, threshold, autoSubmitInt)
	return err
}

// CountTriggersToday returns commit and timer trigger counts for today.
func (d *Database) CountTriggersToday() (commits int, timers int) {
	today := time.Now().Format("2006-01-02")
	d.db.QueryRow(`SELECT COUNT(*) FROM triggers WHERE trigger_type='commit' AND date(timestamp)=?`, today).Scan(&commits)
	d.db.QueryRow(`SELECT COUNT(*) FROM triggers WHERE trigger_type='timer'  AND date(timestamp)=?`, today).Scan(&timers)
	return
}

// GetUnreadNotifications returns the N most recent unread notifications.
func (d *Database) GetUnreadNotifications(limit int) ([]NotificationRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, source, event_type, ticket_id, title,
		       COALESCE(body,''), COALESCE(url,''), read, created_at
		FROM notifications WHERE read=0
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotifications(rows)
}

// GetAllNotifications returns the N most recent notifications (read + unread).
func (d *Database) GetAllNotifications(limit int) ([]NotificationRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, source, event_type, ticket_id, title,
		       COALESCE(body,''), COALESCE(url,''), read, created_at
		FROM notifications
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotifications(rows)
}

// InsertNotification inserts a notification (duplicate ticket_id+event_type+title are ignored).
func (d *Database) InsertNotification(r NotificationRecord) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO notifications (source, event_type, ticket_id, title, body, url)
		VALUES (?,?,?,?,?,?)`,
		r.Source, r.EventType, r.TicketID, r.Title, r.Body, r.URL)
	return err
}

// InsertNotificationNew inserts a notification and returns true when the row was
// actually inserted (i.e. not a duplicate of an existing ticket_id+event_type+title).
func (d *Database) InsertNotificationNew(r NotificationRecord) (bool, error) {
	result, err := d.db.Exec(`
		INSERT OR IGNORE INTO notifications (source, event_type, ticket_id, title, body, url)
		VALUES (?,?,?,?,?,?)`,
		r.Source, r.EventType, r.TicketID, r.Title, r.Body, r.URL)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

// MarkAllNotificationsRead marks every unread notification as read.
func (d *Database) MarkAllNotificationsRead() error {
	_, err := d.db.Exec(`UPDATE notifications SET read=1 WHERE read=0`)
	return err
}

func scanNotifications(rows interface {
	Next() bool
	Scan(...any) error
	Close() error
}) ([]NotificationRecord, error) {
	defer rows.Close()
	var out []NotificationRecord
	for rows.Next() {
		var r NotificationRecord
		var createdAt string
		var readInt int
		if err := rows.Scan(&r.ID, &r.Source, &r.EventType, &r.TicketID, &r.Title, &r.Body, &r.URL, &readInt, &createdAt); err != nil {
			continue
		}
		r.Read = readInt != 0
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		out = append(out, r)
	}
	return out, nil
}

// GetAlertLastChecked returns the last-checked time for a (userID, source) pair.
// Returns zero time and false when no record exists.
func (d *Database) GetAlertLastChecked(userID, source string) (time.Time, bool, error) {
	key := userID + ":" + source
	var ts string
	err := d.db.QueryRow(
		`SELECT last_checked FROM alert_state WHERE id=?`, key,
	).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	t, parseErr := time.Parse(time.RFC3339, ts)
	if parseErr != nil {
		t, parseErr = time.Parse("2006-01-02 15:04:05", ts)
	}
	if parseErr != nil {
		return time.Time{}, false, parseErr
	}
	return t.UTC(), true, nil
}

// SetAlertLastChecked persists the last-checked timestamp for a (userID, source) pair.
func (d *Database) SetAlertLastChecked(userID, source string, ts time.Time) error {
	key := userID + ":" + source
	_, err := d.db.Exec(
		`INSERT INTO alert_state (id, user_id, source, last_checked, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		     last_checked=excluded.last_checked,
		     updated_at=CURRENT_TIMESTAMP`,
		key, userID, source, ts.UTC().Format(time.RFC3339),
	)
	return err
}

// --- Ticket Cache ---

// TicketCacheRecord represents a cached ticket from an upstream PM system.
type TicketCacheRecord struct {
	ID          string
	Source      string
	ExternalID  string
	Repo        string
	Title       string
	Description string
	Status      string
	Assignee    string
	Labels      string // raw JSON array
	URL         string
	SyncedAt    time.Time
	CreatedAt   time.Time
}

// PMUpdateQueueRecord represents a pending PM API call waiting to be dispatched.
type PMUpdateQueueRecord struct {
	ID         int64
	TicketID   string
	Action     string
	Payload    string // raw JSON
	CommitHash string
	Status     string
	Attempts   int
	LastError  string
	CreatedAt  time.Time
	SentAt     *time.Time
}

// UpsertTicketCache inserts or replaces a ticket_cache row.
func (d *Database) UpsertTicketCache(record TicketCacheRecord) error {
	query := `
		INSERT OR REPLACE INTO ticket_cache
			(id, source, external_id, repo, title, description, status, assignee, labels, url, synced_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(
			(SELECT created_at FROM ticket_cache WHERE id = ?),
			CURRENT_TIMESTAMP
		))
	`
	_, err := d.db.Exec(query,
		record.ID,
		record.Source,
		record.ExternalID,
		record.Repo,
		record.Title,
		record.Description,
		record.Status,
		record.Assignee,
		record.Labels,
		record.URL,
		record.SyncedAt,
		record.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert ticket cache: %w", err)
	}
	return nil
}

// GetTicketsByAssignee returns all cached tickets for a given assignee.
func (d *Database) GetTicketsByAssignee(assignee string) ([]TicketCacheRecord, error) {
	query := `
		SELECT id, source, external_id, COALESCE(repo,''), title,
		       COALESCE(description,''), COALESCE(status,''), COALESCE(assignee,''),
		       COALESCE(labels,''), COALESCE(url,''), synced_at, created_at
		FROM ticket_cache
		WHERE assignee = ?
		ORDER BY synced_at DESC
	`
	rows, err := d.db.Query(query, assignee)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickets by assignee: %w", err)
	}
	defer rows.Close()

	var tickets []TicketCacheRecord
	for rows.Next() {
		var r TicketCacheRecord
		var syncedAt, createdAt string
		if err := rows.Scan(
			&r.ID, &r.Source, &r.ExternalID, &r.Repo, &r.Title,
			&r.Description, &r.Status, &r.Assignee, &r.Labels, &r.URL,
			&syncedAt, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}
		r.SyncedAt, _ = time.Parse("2006-01-02 15:04:05", syncedAt)
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		tickets = append(tickets, r)
	}
	return tickets, nil
}

// GetTicketByID returns a single cached ticket by its composite ID, or nil if not found.
func (d *Database) GetTicketByID(id string) (*TicketCacheRecord, error) {
	query := `
		SELECT id, source, external_id, COALESCE(repo,''), title,
		       COALESCE(description,''), COALESCE(status,''), COALESCE(assignee,''),
		       COALESCE(labels,''), COALESCE(url,''), synced_at, created_at
		FROM ticket_cache
		WHERE id = ?
	`
	var r TicketCacheRecord
	var syncedAt, createdAt string
	err := d.db.QueryRow(query, id).Scan(
		&r.ID, &r.Source, &r.ExternalID, &r.Repo, &r.Title,
		&r.Description, &r.Status, &r.Assignee, &r.Labels, &r.URL,
		&syncedAt, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket by id: %w", err)
	}
	r.SyncedAt, _ = time.Parse("2006-01-02 15:04:05", syncedAt)
	r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &r, nil
}

// --- PM Update Queue ---

// InsertPMUpdateQueue inserts a new PM update request into the outbox queue.
// Returns the row ID of the inserted record.
func (d *Database) InsertPMUpdateQueue(record PMUpdateQueueRecord) (int64, error) {
	query := `
		INSERT INTO pm_update_queue (ticket_id, action, payload, commit_hash, status, attempts)
		VALUES (?, ?, ?, ?, 'pending', 0)
	`
	result, err := d.db.Exec(query,
		record.TicketID,
		record.Action,
		record.Payload,
		record.CommitHash,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert pm update queue: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return id, nil
}

// GetPendingPMUpdates returns all rows in pm_update_queue with status = 'pending'.
func (d *Database) GetPendingPMUpdates() ([]PMUpdateQueueRecord, error) {
	query := `
		SELECT id, ticket_id, action, payload, COALESCE(commit_hash,''),
		       status, attempts, COALESCE(last_error,''), created_at, sent_at
		FROM pm_update_queue
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending pm updates: %w", err)
	}
	defer rows.Close()

	var updates []PMUpdateQueueRecord
	for rows.Next() {
		var r PMUpdateQueueRecord
		var createdAt string
		var sentAt sql.NullString
		if err := rows.Scan(
			&r.ID, &r.TicketID, &r.Action, &r.Payload, &r.CommitHash,
			&r.Status, &r.Attempts, &r.LastError, &createdAt, &sentAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pm update: %w", err)
		}
		r.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		if sentAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", sentAt.String)
			r.SentAt = &t
		}
		updates = append(updates, r)
	}
	return updates, nil
}

// MarkPMUpdateSent marks a pm_update_queue row as successfully sent.
func (d *Database) MarkPMUpdateSent(id int64) error {
	query := `
		UPDATE pm_update_queue
		SET status = 'sent', sent_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark pm update as sent: %w", err)
	}
	return nil
}

// Exec runs a raw SQL statement on the underlying database connection.
// Prefer dedicated methods; this escape hatch exists only for inline queries
// in package main that cannot be refactored into a typed method without
// major churn (e.g. auto-stop work sessions, expire deferred commits).
func (d *Database) Exec(query string, args ...any) (sql.Result, error) {
	return d.db.Exec(query, args...)
}

// ExecRaw executes a raw SQL statement. Used by tests and migrations only.
// Identical to Exec; provided under a distinct name so call sites in tests are
// clearly distinct from production call sites.
func (d *Database) ExecRaw(query string, args ...any) (sql.Result, error) {
	return d.db.Exec(query, args...)
}

// NewDatabaseAtPath opens a SQLite database at an explicit path without reading
// any environment variables. Intended for use in tests that cannot set up the
// full environment, and for temporary databases in other contexts.
// It applies both the base schema (initSchema) and the migration-managed tables
// so the resulting DB is fully functional without requiring RunPendingMigrations.
func NewDatabaseAtPath(dbPath string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	d := &Database{db: sqlDB, path: dbPath}
	if err := d.initSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	// Apply migration-managed tables inline so the DB is fully functional
	// without requiring the full env-var setup that RunPendingMigrations needs.
	if err := d.applyMigrationTables(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to apply migration tables: %w", err)
	}
	return d, nil
}

// applyMigrationTables creates the tables that are normally applied by
// RunPendingMigrations but are not in the base initSchema. Called by
// NewDatabaseAtPath so test databases are fully functional without env vars.
func (d *Database) applyMigrationTables() error {
	stmts := []string{
		// 006-create-pending-actions
		`CREATE TABLE IF NOT EXISTS pending_actions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			action_type TEXT    NOT NULL,
			target      TEXT    NOT NULL,
			platform    TEXT    NOT NULL,
			workspace   TEXT    NOT NULL,
			payload     TEXT    NOT NULL,
			confidence  REAL    NOT NULL,
			status      TEXT    NOT NULL DEFAULT 'pending',
			expires_at  DATETIME NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			acted_at    DATETIME,
			acted_by    TEXT,
			error       TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_actions_status ON pending_actions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_actions_expires ON pending_actions(expires_at)`,
		// 008-create-inferences-fts5 (plain table only; FTS5 virtual table omitted for test simplicity)
		`CREATE TABLE IF NOT EXISTS inferences (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			context_type TEXT    NOT NULL,
			subject      TEXT    NOT NULL,
			inference    TEXT    NOT NULL,
			evidence     TEXT    NOT NULL,
			confidence   REAL    NOT NULL DEFAULT 0.5,
			source       TEXT    NOT NULL DEFAULT 'hermes3',
			created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		// 009-create-corrections (needed by voice profile tools)
		`CREATE TABLE IF NOT EXISTS corrections (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			inference_id INTEGER NOT NULL,
			correction   TEXT    NOT NULL,
			flagged_from TEXT    NOT NULL,
			weight       REAL    NOT NULL DEFAULT 1.0,
			created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		// 010-create-confidence-thresholds
		`CREATE TABLE IF NOT EXISTS confidence_thresholds (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			action_type  TEXT    NOT NULL,
			workspace    TEXT    NOT NULL DEFAULT '',
			threshold    REAL    NOT NULL DEFAULT 0.70,
			approvals    INTEGER NOT NULL DEFAULT 0,
			rejections   INTEGER NOT NULL DEFAULT 0,
			last_updated DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(action_type, workspace)
		)`,
		// 011-create-skills
		`CREATE TABLE IF NOT EXISTS skills (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT    NOT NULL UNIQUE,
			description    TEXT    NOT NULL,
			context_type   TEXT    NOT NULL,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			promoted_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			last_seen_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		// 012-create-pr-review-comments
		`CREATE TABLE IF NOT EXISTS pr_review_comments (
			platform      TEXT     NOT NULL,
			comment_id    TEXT     NOT NULL,
			pr_id         TEXT     NOT NULL,
			workspace     TEXT     NOT NULL,
			status        TEXT     NOT NULL DEFAULT 'new',
			comment_body  TEXT     NOT NULL DEFAULT '',
			classified_as TEXT,
			fix_hint      TEXT     NOT NULL DEFAULT '',
			created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			attempt_count INTEGER  NOT NULL DEFAULT 0,
			PRIMARY KEY (platform, comment_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pr_comments_status ON pr_review_comments(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pr_comments_pr     ON pr_review_comments(pr_id, platform)`,
	}
	for _, stmt := range stmts {
		if _, err := d.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("applyMigrationTables: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// MCP read-only query helpers (TASK-099)
// ---------------------------------------------------------------------------

// TriggerCommit is a minimal commit record returned by MCP tools.
// It is a lighter-weight view than the full TriggerRecord.
// The triggers table stores repo_path (not workspace_name) and has no branch
// column; those fields are populated from available data.
type TriggerCommit struct {
	Hash      string
	Message   string
	TicketID  string
	RepoPath  string // maps to triggers.repo_path
	Timestamp string
}

// ListTodayCommits returns all commit triggers from today (UTC date), optionally
// filtered to a specific repo path. Pass repoPath="" for all repos.
// Results are ordered ASC by timestamp.
func (d *Database) ListTodayCommits(repoPath string) ([]TriggerCommit, error) {
	q := `
		SELECT COALESCE(commit_hash,''), COALESCE(commit_message,''),
		       COALESCE(ticket_id,''), COALESCE(repo_path,''), COALESCE(timestamp,'')
		FROM triggers
		WHERE trigger_type='commit'
		  AND date(timestamp) = date('now')
		  AND (? = '' OR repo_path = ?)
		ORDER BY timestamp ASC
	`
	rows, err := d.db.Query(q, repoPath, repoPath)
	if err != nil {
		return nil, fmt.Errorf("ListTodayCommits: %w", err)
	}
	defer rows.Close()
	var out []TriggerCommit
	for rows.Next() {
		var c TriggerCommit
		if err := rows.Scan(&c.Hash, &c.Message, &c.TicketID, &c.RepoPath, &c.Timestamp); err != nil {
			return nil, fmt.Errorf("ListTodayCommits scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListTicketCommits returns the N most recent commit triggers for a given ticket_id.
func (d *Database) ListTicketCommits(ticketID string, limit int) ([]TriggerCommit, error) {
	q := `
		SELECT COALESCE(commit_hash,''), COALESCE(commit_message,''),
		       COALESCE(ticket_id,''), COALESCE(repo_path,''), COALESCE(timestamp,'')
		FROM triggers
		WHERE trigger_type='commit'
		  AND ticket_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := d.db.Query(q, ticketID, limit)
	if err != nil {
		return nil, fmt.Errorf("ListTicketCommits: %w", err)
	}
	defer rows.Close()
	var out []TriggerCommit
	for rows.Next() {
		var c TriggerCommit
		if err := rows.Scan(&c.Hash, &c.Message, &c.TicketID, &c.RepoPath, &c.Timestamp); err != nil {
			return nil, fmt.Errorf("ListTicketCommits scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MostRecentCommit returns the most recent commit trigger across all repos.
// Returns a zero TriggerCommit (empty fields) when no commits exist.
func (d *Database) MostRecentCommit() (TriggerCommit, error) {
	var c TriggerCommit
	err := d.db.QueryRow(`
		SELECT COALESCE(commit_hash,''), COALESCE(commit_message,''),
		       COALESCE(ticket_id,''), COALESCE(repo_path,''), COALESCE(timestamp,'')
		FROM triggers
		WHERE trigger_type='commit'
		ORDER BY timestamp DESC
		LIMIT 1
	`).Scan(&c.Hash, &c.Message, &c.TicketID, &c.RepoPath, &c.Timestamp)
	if err == sql.ErrNoRows {
		return TriggerCommit{}, nil
	}
	if err != nil {
		return TriggerCommit{}, fmt.Errorf("MostRecentCommit: %w", err)
	}
	return c, nil
}

// CountTodayCommits returns the number of commit triggers today (UTC date).
func (d *Database) CountTodayCommits() (int, error) {
	var n int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM triggers
		WHERE trigger_type='commit'
		  AND date(timestamp) = date('now')
	`).Scan(&n)
	return n, err
}

// MarkPMUpdateFailed increments attempts and records the error on a pm_update_queue row.
// The row stays 'pending' to allow future retries.
func (d *Database) MarkPMUpdateFailed(id int64, errMsg string) error {
	query := `
		UPDATE pm_update_queue
		SET attempts = attempts + 1, last_error = ?
		WHERE id = ?
	`
	_, err := d.db.Exec(query, errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to mark pm update as failed: %w", err)
	}
	return nil
}
