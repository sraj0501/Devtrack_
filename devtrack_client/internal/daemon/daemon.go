package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/alerts"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/health"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/infra"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/notify"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/reviewer"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/telegram"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// init wires the platform-specific process-alive check into internal/health.
// CheckProcessAlive is defined in process_unix.go / process_windows.go.
func init() {
	health.IsProcessAlive = CheckProcessAlive
}

// newHealthMonitor builds a HealthMonitor with the platform/transport wiring
// the daemon needs: a server-ping closure (via the HTTP trigger client) and the
// configured server URL. Previously lived in package main's health_shim.go.
func newHealthMonitor(database *db.Database) *health.HealthMonitor {
	hm := health.NewHealthMonitor(database)
	hm.PingServerFn = func() bool { return trigger.NewHTTPTriggerClient().Ping() }
	hm.GetServerURL = config.GetServerURL
	return hm
}

// Daemon manages the background process lifecycle
type Daemon struct {
	monitor       *infra.IntegratedMonitor
	config        *config.Config
	pidFile       string
	logFile       string
	lockFile      *os.File // exclusive lock — held for process lifetime
	ctx           context.Context
	cancel        context.CancelFunc
	isRunning     bool
	webhookServer *exec.Cmd
	webhookMu     sync.Mutex
	alertPoller   *alerts.Poller // native Go alert poller (Phase 2)
	telegramBot   *telegram.Bot  // interactive Telegram bot (Phase 3)
	startTime     time.Time
	healthMonitor *health.HealthMonitor
	prLoopGuard   sync.Map // guards one PRFixLoop goroutine per "platform:prID"

	// TicketSyncFns are set by package main (which owns the connector code).
	// PushCachedFn reads from local SQLite and pushes to Python (no API call).
	// FullSyncFn fetches from PM APIs, updates local SQLite, then pushes.
	PushCachedFn func()
	FullSyncFn   func()
}

// DaemonStatus represents the current daemon state
type DaemonStatus struct {
	Running      bool
	PID          int
	Uptime       time.Duration
	StartTime    time.Time
	ConfigPath   string
	LogPath      string
	PIDPath      string
	TriggerCount int
	LastTrigger  time.Time
}

// NewDaemon creates a new daemon instance
func NewDaemon(repoPath string) (*Daemon, error) {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := os.MkdirAll(config.GetPIDDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create PID directory: %w", err)
	}

	if err := os.MkdirAll(config.GetLogDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	daemon := &Daemon{
		config:  cfg,
		pidFile: config.GetPIDFilePath(),
		logFile: config.GetLogFilePath(),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Create integrated monitor
	monitor, err := infra.NewIntegratedMonitor(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitor: %w", err)
	}

	daemon.monitor = monitor

	return daemon, nil
}

// Monitor returns the daemon's integrated monitor (nil until NewDaemon succeeds).
// Exported so the CLI can query the scheduler/database through it.
func (d *Daemon) Monitor() *infra.IntegratedMonitor { return d.monitor }

// Start starts the daemon process
func (d *Daemon) Start() error {
	// Acquire exclusive lock first — atomic guard against multiple instances.
	// The OS releases the lock automatically when this process exits for any reason,
	// so a crashed daemon never leaves a stale lock behind.
	lockPath := filepath.Join(config.GetPIDDir(), "devtrack.lock")
	lf, err := acquireDaemonLock(lockPath)
	if err != nil {
		return fmt.Errorf("daemon already running (could not acquire lock)")
	}
	d.lockFile = lf

	// Setup logging
	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	log.Println("Starting DevTrack daemon...")
	log.Printf("PID file: %s", d.pidFile)
	log.Printf("Log file: %s", d.logFile)
	log.Printf("config.Config: %s", config.GetConfigPath())

	// Write PID file
	if err := d.writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Generate TLS cert before starting monitoring or any Python subprocess.
	// Must run before d.monitor.Start(): the queue executor constructs a
	// long-lived trigger.HTTPTriggerClient that pins the cert once, at
	// construction time (internal/infra/integrated.go). On a fresh install
	// with no pre-existing cert, starting the monitor first meant that
	// client permanently fell back to system CA roots — which never trust
	// a self-signed cert — breaking the pending-actions queue for the
	// entire lifetime of every first-ever daemon run.
	if config.IsTLSEnabled() {
		if err := d.generateTLSCert(); err != nil {
			log.Printf("Warning: TLS cert generation failed (%v) — disabling TLS", err)
			// Force TLS off so subsequent HTTP client builds don't fail
			os.Setenv("DEVTRACK_TLS", "false")
		}
	}

	// Start integrated monitoring (passes d.ctx so the queue executor exits cleanly on stop)
	if err := d.monitor.Start(d.ctx); err != nil {
		d.cleanup()
		return fmt.Errorf("failed to start monitoring: %w", err)
	}

	// Watch workspaces.yaml for changes — triggers hot-reload automatically
	d.startWorkspacesFileWatcher()

	// Start webhook server (primary Python process in CS-1)
	if err := d.startWebhookServer(); err != nil {
		log.Printf("Warning: Failed to start webhook server: %v", err)
		log.Println("HTTP trigger functionality will be unavailable")
	} else {
		// Wait up to 30 s for the Python HTTP server to become healthy. A cold
		// start (LLM task parser, description enhancer, task matcher init) measured
		// ~14s on a fresh managed install; 10s produced a false-alarm warning
		// on every first run even though the server came up fine moments later.
		d.waitForPythonHTTP(30)
	}
	if config.IsExternalServer() {
		log.Printf("External mode: AI triggers will be sent to %s (set DEVTRACK_SERVER_URL to target another host)", config.GetServerURL())
	}

	// Start interactive Telegram bot first so alert poller can use it as notifier.
	d.startTelegramBot()

	// Start native Go alert poller (Phase 2 — replaces Python assignment/telegram/slack subprocesses)
	d.startAlertPoller()

	d.isRunning = true
	d.startTime = time.Now()
	log.Println("✓ Daemon started successfully")

	// Start health monitor
	hm := newHealthMonitor(d.monitor.Database())
	if d.webhookServer != nil && d.webhookServer.Process != nil {
		hm.SetWebhookPID(d.webhookServer.Process.Pid)
	}
	hm.SetRestartCallbacks(nil, d.restartWebhookServer)
	hm.Start()
	d.healthMonitor = hm
	log.Println("✓ Health monitor started")

	// Start internal HTTP server for platform-agnostic control endpoints.
	d.startInternalHTTPServer()

	// Ticket sync: push cached tickets immediately (no API call), then
	// run a full sync + push on a periodic interval.
	d.startTicketSyncLoop()

	// Heartbeat: register this client with the server on startup and every 60s.
	d.startHeartbeatLoop()

	// First-run voice mining waits for the optional AI server and runs once in
	// the background. Go-native monitoring and MCP are already available.
	d.startFirstRunWow()

	// Setup signal handlers for graceful shutdown
	d.setupSignalHandlers()

	// Keep daemon running
	<-d.ctx.Done()

	// Cleanup on shutdown
	log.Println("Shutting down daemon...")
	d.Stop()

	return nil
}

// Stop stops the daemon gracefully
func (d *Daemon) Stop() error {
	if !d.isRunning && !d.IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	log.Println("Stopping daemon...")

	// Stop health monitor
	if d.healthMonitor != nil {
		d.healthMonitor.Stop()
	}

	// Stop Go alert poller
	if d.alertPoller != nil {
		d.alertPoller.Stop()
	}

	// Stop Telegram bot
	if d.telegramBot != nil {
		d.telegramBot.Stop()
	}

	// Stop webhook server (and its Python child on Windows, where "uv run"
	// does not exec-replace itself — see KillProcessTree).
	d.webhookMu.Lock()
	if d.webhookServer != nil {
		log.Println("Stopping webhook server...")
		if err := KillProcessTree(d.webhookServer.Process.Pid); err != nil {
			log.Printf("Warning: error stopping webhook server: %v", err)
		}
	}
	d.webhookMu.Unlock()

	// Stop monitoring
	if d.monitor != nil {
		d.monitor.Stop()
	}

	// Cancel context
	if d.cancel != nil {
		d.cancel()
	}

	// Cleanup
	d.cleanup()

	d.isRunning = false
	log.Println("✓ Daemon stopped")

	return nil
}

// Restart restarts the daemon
func (d *Daemon) Restart() error {
	log.Println("Restarting daemon...")

	// Stop if running
	if d.IsRunning() {
		if err := d.Stop(); err != nil {
			log.Printf("Warning: error during stop: %v", err)
		}
		// Wait a moment for cleanup
		time.Sleep(1 * time.Second)
	}

	// Start again
	return d.Start()
}

// Status returns the current daemon status
func (d *Daemon) Status() (*DaemonStatus, error) {
	status := &DaemonStatus{
		ConfigPath: config.GetConfigPath(),
		LogPath:    d.logFile,
		PIDPath:    d.pidFile,
	}

	// Check if running
	status.Running = d.IsRunning()

	if status.Running {
		// Read PID
		pid, err := d.ReadPID()
		if err == nil {
			status.PID = pid
		}

		// Get monitoring stats if available
		if d.monitor != nil && d.monitor.Scheduler() != nil {
			stats := d.monitor.Scheduler().GetStats()
			if count, ok := stats["trigger_count"].(int); ok {
				status.TriggerCount = count
			}
			if lastTrigger, ok := stats["last_trigger"].(time.Time); ok {
				status.LastTrigger = lastTrigger
			}
		}

		// Calculate uptime from daemon start time
		if !d.startTime.IsZero() {
			status.StartTime = d.startTime
			status.Uptime = time.Since(d.startTime)
		} else if info, err := os.Stat(d.pidFile); err == nil {
			// PID file is written once at startup — its mtime is the start time
			status.StartTime = info.ModTime()
			status.Uptime = time.Since(status.StartTime)
		}
	}

	return status, nil
}

// IsRunning checks if the daemon is currently running
func (d *Daemon) IsRunning() bool {
	pid, err := d.ReadPID()
	if err != nil {
		return false
	}

	return CheckProcessAlive(pid)
}

// Pause pauses the scheduler (but keeps daemon running)
func (d *Daemon) Pause() error {
	if !d.IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	if d.monitor != nil && d.monitor.Scheduler() != nil {
		d.monitor.Scheduler().Pause()
		log.Println("✓ infra.Scheduler paused")
		return nil
	}

	return fmt.Errorf("scheduler not available")
}

// Resume resumes the scheduler
func (d *Daemon) Resume() error {
	if !d.IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	if d.monitor != nil && d.monitor.Scheduler() != nil {
		d.monitor.Scheduler().Resume()
		log.Println("✓ infra.Scheduler resumed")
		return nil
	}

	return fmt.Errorf("scheduler not available")
}

// setupLogging configures logging to file
func (d *Daemon) setupLogging() error {
	// Create log file
	logFile, err := os.OpenFile(d.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Redirect log output to file
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return nil
}

// setupSignalHandlers is platform-specific — see daemon_unix.go and daemon_windows.go.

// writePID writes the current process ID to the PID file
func (d *Daemon) writePID() error {
	pid := os.Getpid()
	return os.WriteFile(d.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// ReadPID reads the PID from the PID file
func (d *Daemon) ReadPID() (int, error) {
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

// cleanup removes PID file and releases the exclusive lock
func (d *Daemon) cleanup() {
	if err := os.Remove(d.pidFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove PID file: %v", err)
	}
	releaseDaemonLock(d.lockFile)
	d.lockFile = nil
}

// GetLogs returns the last N lines from the log file
func (d *Daemon) GetLogs(lines int) ([]string, error) {
	data, err := os.ReadFile(d.logFile)
	if err != nil {
		return nil, err
	}

	// Split into lines
	allLines := []string{}
	currentLine := ""
	for _, b := range data {
		if b == '\n' {
			if currentLine != "" {
				allLines = append(allLines, currentLine)
			}
			currentLine = ""
		} else {
			currentLine += string(b)
		}
	}
	if currentLine != "" {
		allLines = append(allLines, currentLine)
	}

	// Return last N lines
	if lines <= 0 || lines > len(allLines) {
		return allLines, nil
	}

	return allLines[len(allLines)-lines:], nil
}

// generateTLSCert ensures a self-signed TLS certificate exists for the Go↔Python
// channel.  Regeneration is skipped when the cert already exists and has at
// least 30 days of validity remaining — this keeps the cert stable across
// daemon restarts so remote clients that have copied the cert don't break.
func (d *Daemon) generateTLSCert() error {
	certPath := config.GetTLSCertPath()
	keyPath := config.GetTLSKeyPath()
	if trigger.CertExistsAndValid(certPath, 30) {
		log.Printf("TLS cert still valid, reusing: %s", certPath)
	} else {
		log.Printf("Generating TLS cert: %s", certPath)
		if err := trigger.GenerateSelfSignedCert(certPath, keyPath); err != nil {
			return fmt.Errorf("TLS cert generation: %w", err)
		}
		log.Println("✓ TLS cert generated")
	}
	// Expose paths so subprocesses pick them up
	os.Setenv("DEVTRACK_TLS_CERT", certPath)
	os.Setenv("DEVTRACK_TLS_KEY", keyPath)
	return nil
}

// waitForPythonHTTP polls /health on the Python server until it responds
// or the timeout (in seconds) elapses.
func (d *Daemon) waitForPythonHTTP(timeoutSecs int) {
	client := trigger.NewHTTPTriggerClient()
	deadline := time.Now().Add(time.Duration(timeoutSecs) * time.Second)
	for time.Now().Before(deadline) {
		if client.Ping() {
			log.Println("✓ Python HTTP server is ready")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("Warning: Python HTTP server did not respond within %ds — triggers may fail until it starts", timeoutSecs)
}

// startWebhookServer starts the Python webhook/trigger server.
// In CS-1 this is the primary Python process; python_bridge.py is no longer spawned.
// When DEVTRACK_SERVER_MODE=external the server is managed by the user.
func (d *Daemon) startWebhookServer() error {
	d.webhookMu.Lock()
	defer d.webhookMu.Unlock()
	return d.startWebhookServerLocked()
}

func (d *Daemon) startWebhookServerLocked() error {
	if config.IsExternalServer() {
		log.Printf("Python backend is externally managed (DEVTRACK_SERVER_MODE=external)")
		log.Printf("  Expected server: %s", config.GetServerURL())
		return nil
	}

	log.Println("Starting Python server (webhook + triggers)...")

	var cmd *exec.Cmd
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot != "" {
		cmd = exec.Command("uv", "run", "--directory", projectRoot, "python", "-m", "backend.webhook_server")
		cmd.Dir = projectRoot
	} else {
		// Try standard managed install location (set up by 'devtrack setup'):
		// $XDG_DATA_HOME/devtrack/server/devtrack_server. Note this is the XDG
		// data home, not DEVTRACK_HOME (a distinct, unrelated env var).
		devtrackHome, homeErr := config.DevtrackDataHome()
		if homeErr != nil {
			return fmt.Errorf("could not determine DevTrack data home: %w", homeErr)
		}
		standardPath := filepath.Join(devtrackHome, "server", "devtrack_server")
		if _, statErr := os.Stat(filepath.Join(standardPath, "backend")); statErr == nil {
			projectRoot = standardPath
			cmd = exec.Command("uv", "run", "--directory", projectRoot, "python", "-m", "backend.webhook_server")
			cmd.Dir = projectRoot
		} else {
			return fmt.Errorf(
				"managed mode: Python server not found at %s. "+
					"Run 'devtrack setup' to install it, or set PROJECT_ROOT env var",
				standardPath,
			)
		}
	}

	// Pass TLS cert paths so uvicorn starts with TLS enabled
	cmd.Env = append(os.Environ(),
		"DEVTRACK_TLS_CERT="+config.GetTLSCertPath(),
		"DEVTRACK_TLS_KEY="+config.GetTLSKeyPath(),
	)

	// Redirect output to daemon log
	logFile, err := os.OpenFile(d.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file for webhook server: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Python server: %w", err)
	}

	d.webhookServer = cmd
	log.Printf("✓ Python server started (PID: %d)", cmd.Process.Pid)

	return nil
}

// restartWebhookServer restarts the webhook server process
func (d *Daemon) restartWebhookServer() error {
	d.webhookMu.Lock()
	defer d.webhookMu.Unlock()
	if d.webhookServer != nil && d.webhookServer.Process != nil {
		KillProcessTree(d.webhookServer.Process.Pid)
		d.webhookServer.Process.Wait()
	}

	if err := d.startWebhookServerLocked(); err != nil {
		return err
	}

	if d.healthMonitor != nil && d.webhookServer != nil && d.webhookServer.Process != nil {
		d.healthMonitor.SetWebhookPID(d.webhookServer.Process.Pid)
	}
	return nil
}

// startAlertPoller starts the native Go ticket alert poller if ALERT_ENABLED=true.
// Polls GitHub and Azure for assigned/comment/review events, writes to SQLite,
// and delivers via terminal + Telegram + Slack notifiers.
func (d *Daemon) startAlertPoller() {
	if !config.IsAlertEnabled() {
		log.Println("Alert poller disabled (ALERT_ENABLED is not true)")
		return
	}

	// Use the already-started Telegram bot as the notifier if available,
	// otherwise fall back to the lightweight HTTP-only notifier.
	var tgNotifier notify.Notifier
	if d.telegramBot != nil {
		tgNotifier = d.telegramBot
	} else {
		tgNotifier = notify.NewTelegramFromConfig()
	}
	notifier := notify.NewMulti(
		notify.Terminal{},
		tgNotifier,
		notify.NewSlackFromConfig(),
		notify.OS{},
	)

	poller := alerts.NewPoller(d.monitor.Database(), notifier)

	// Wire Phase 7 review comment classification hook.
	// After the poller detects new review comments, classify each one via
	// the Python server and update the pr_review_comments row.
	database := d.monitor.Database()
	poller.SetReviewCommentHook(func(events []alerts.ReviewCommentEvent) {
		tc := trigger.NewHTTPTriggerClient()
		for _, ev := range events {
			classification, reason, fixHint, err := tc.ClassifyReviewComment(
				ev.CommentBody, ev.PRTitle, ev.Platform, ev.CommentURL,
			)
			if err != nil {
				log.Printf("review: classify comment %s on PR %s: %v — defaulting to needs_human",
					ev.CommentID, ev.PRID, err)
				classification = "needs_human"
				reason = "classification error"
				fixHint = ""
			}
			if dbErr := database.UpdatePRReviewCommentStatus(
				ev.Platform, ev.CommentID, "classified", classification, fixHint,
			); dbErr != nil {
				log.Printf("review: update comment status %s: %v", ev.CommentID, dbErr)
			}
			log.Printf("review: comment %s on PR %s classified as %s (%s)",
				ev.CommentID, ev.PRID, classification, reason)
			if classification == "auto_fixable" {
				loopKey := ev.Platform + ":" + ev.PRID
				if _, alreadyRunning := d.prLoopGuard.LoadOrStore(loopKey, true); !alreadyRunning {
					go func(event alerts.ReviewCommentEvent) {
						defer d.prLoopGuard.Delete(loopKey)
						ag := reviewer.NewAgent(
							reviewer.AgentBackend(config.GetReviewAgent()),
							config.GetReviewAgentTimeoutSecs(),
						)
						// Nil-interface care: NewApprovalChecker returns a typed nil for
						// unsupported platforms — never assign it to the interface directly.
						var checker reviewer.PRApprovalChecker
						if c := alerts.NewApprovalChecker(event.Platform); c != nil {
							checker = c
						}
						loop := reviewer.NewPRFixLoop(database, ag, checker)
						report := loop.Run(d.ctx, event.Platform, event.PRID, event.Workspace, "")

						// Count fixes applied so far for this PR.
						allComments, _ := database.ListPRReviewCommentsByPR(event.Platform, event.PRID)
						fixesApplied := 0
						for _, c := range allComments {
							if c.Status == "fix_applied" {
								fixesApplied++
							}
						}
						prTitle := event.PRTitle
						prURL := prURLFromCommentURL(event.CommentURL)

						if report.Stuck {
							// Stage pr_escalation pending action (Non-Negotiable #2: queue first).
							payload, _ := json.Marshal(map[string]any{
								"pr_title":       prTitle,
								"pr_id":          event.PRID,
								"blocker_reason": report.BlockerReason,
								"comment_url":    report.CommentURL,
								"fixes_applied":  fixesApplied,
								"pr_url":         prURL,
							})
							_, _ = database.InsertPendingAction(db.PendingAction{
								ActionType: "pr_escalation",
								Target:     event.Platform + ":PR #" + event.PRID,
								Platform:   event.Platform,
								Workspace:  event.Workspace,
								Confidence: 1.0,
								Payload:    string(payload),
								Status:     "pending",
								ExpiresAt:  time.Now().Add(db.ConfidenceTimeout(1.0, false)),
							})
							// Channel parity: send Telegram immediately alongside queue insert.
							if d.telegramBot != nil {
								_ = d.telegramBot.SendPREscalation(prTitle, report.BlockerReason, report.CommentURL, prURL)
							}
							log.Printf("review: staged pr_escalation for PR %s — %s", event.PRID, report.BlockerReason)
						} else {
							// Stage pr_approved_notify pending action.
							payload, _ := json.Marshal(map[string]any{
								"pr_title":      prTitle,
								"pr_id":         event.PRID,
								"fixes_applied": fixesApplied,
								"pr_url":        prURL,
							})
							_, _ = database.InsertPendingAction(db.PendingAction{
								ActionType: "pr_approved_notify",
								Target:     event.Platform + ":PR #" + event.PRID,
								Platform:   event.Platform,
								Workspace:  event.Workspace,
								Confidence: 1.0,
								Payload:    string(payload),
								Status:     "pending",
								ExpiresAt:  time.Now().Add(db.ConfidenceTimeout(1.0, false)),
							})
							// Channel parity: send Telegram immediately alongside queue insert.
							if d.telegramBot != nil {
								_ = d.telegramBot.SendPRApproved(prTitle, prURL)
							}
							log.Printf("review: staged pr_approved_notify for PR %s", event.PRID)
						}
					}(ev)
				}
			} else {
				// needs_human: escalate immediately — no fix loop will run.
				prTitle := ev.PRTitle
				blockerReason := "comment needs human review"
				commentURL := ev.CommentURL
				prURL := prURLFromCommentURL(commentURL)

				payload, _ := json.Marshal(map[string]any{
					"pr_title":       prTitle,
					"pr_id":          ev.PRID,
					"blocker_reason": blockerReason,
					"comment_url":    commentURL,
					"fixes_applied":  0,
					"pr_url":         prURL,
				})
				_, _ = database.InsertPendingAction(db.PendingAction{
					ActionType: "pr_escalation",
					Target:     ev.Platform + ":PR #" + ev.PRID,
					Platform:   ev.Platform,
					Workspace:  ev.Workspace,
					Confidence: 1.0,
					Payload:    string(payload),
					Status:     "pending",
					ExpiresAt:  time.Now().Add(db.ConfidenceTimeout(1.0, false)),
				})
				if d.telegramBot != nil {
					_ = d.telegramBot.SendPREscalation(prTitle, blockerReason, commentURL, prURL)
				}
				// Update comment status to "escalated" so the review CLI shows it.
				_ = database.UpdatePRReviewCommentStatus(ev.Platform, ev.CommentID, "escalated", classification, fixHint)
				log.Printf("review: staged pr_escalation (needs_human) for PR %s comment %s", ev.PRID, ev.CommentID)
			}
		}
	})

	// TASK-126: merged-PR hook — a PR authored by the developer merged into the
	// default branch means the ticket's work is done. Convert each event into a
	// commit trigger with is_merge_to_default=true; the Python server stages the
	// done state-transition in the pending-actions queue (never posts directly).
	poller.SetMergedPRHook(func(events []alerts.MergedPREvent) {
		wsCfg, err := config.LoadWorkspacesConfig()
		if err != nil || wsCfg == nil {
			log.Printf("merged-pr: load workspaces: %v", err)
			return
		}
		tc := trigger.NewHTTPTriggerClient()
		for _, ev := range events {
			var ws *config.WorkspaceConfig
			for i := range wsCfg.Workspaces {
				if wsCfg.Workspaces[i].Name == ev.Workspace {
					ws = &wsCfg.Workspaces[i]
					break
				}
			}
			if ws == nil {
				log.Printf("merged-pr: no workspace %q for PR %s, skipping", ev.Workspace, ev.PRID)
				continue
			}
			ext, _ := ticket.NewExtractor(ws.TicketPattern)
			ticketConfidence := 0.95 // merged branch name — developer contract
			ticketID := ext.Extract(ev.HeadBranch)
			if ticketID == "" {
				ticketID = ext.Extract(ev.PRTitle)
				ticketConfidence = 0.85
			}
			if ticketID == "" {
				log.Printf("[UNLINKED] merged PR %s (%q → %q) workspace=%q — no ticket ID extracted",
					ev.PRID, ev.HeadBranch, ev.BaseBranch, ev.Workspace)
				continue
			}
			err := tc.SendCommitTrigger(trigger.CommitTriggerData{
				RepoPath:          ws.Path,
				CommitHash:        ev.MergeSHA,
				CommitMessage:     fmt.Sprintf("Merge PR #%s: %s (branch %s)", ev.PRID, ev.PRTitle, ev.HeadBranch),
				Timestamp:         ev.MergedAt.Format(time.RFC3339),
				Branch:            ev.BaseBranch,
				TicketID:          ticketID,
				TicketConfidence:  ticketConfidence,
				IsMergeToDefault:  true,
				WorkspaceName:     ws.Name,
				PMPlatform:        ws.PMPlatform,
				PMProject:         ws.PMProject,
				PMAssignee:        ws.PMAssignee,
				PMIterationPath:   ws.PMIterationPath,
				PMAreaPath:        ws.PMAreaPath,
				PMMilestone:       ws.PMMilestone,
				PMInProgressLabel: ws.InProgressLabel,
			})
			if err != nil {
				log.Printf("merged-pr: send trigger for PR %s (ticket %s): %v", ev.PRID, ticketID, err)
				continue
			}
			log.Printf("merged-pr: PR %s merged into %s → staged done transition for ticket %s",
				ev.PRID, ev.BaseBranch, ticketID)
		}
	})

	poller.Start(d.ctx)
	d.alertPoller = poller
}

// prURLFromCommentURL derives the PR web URL from a review comment URL by stripping
// the fragment identifier (the "#discussion_r..." suffix on GitHub comment URLs).
// Returns the input unchanged when no fragment is present.
func prURLFromCommentURL(commentURL string) string {
	if i := strings.Index(commentURL, "#"); i >= 0 {
		return commentURL[:i]
	}
	return commentURL
}

// startWorkspacesFileWatcher watches workspaces.yaml for changes and triggers
// a hot-reload automatically — no CLI command or SIGHUP needed.
func (d *Daemon) startWorkspacesFileWatcher() {
	wsFile := config.GetWorkspacesFilePath()
	if _, err := os.Stat(wsFile); os.IsNotExist(err) {
		log.Println("workspaces.yaml not found — file watcher skipped (single-repo mode)")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Warning: could not start workspaces file watcher: %v", err)
		return
	}
	if err := watcher.Add(wsFile); err != nil {
		watcher.Close()
		log.Printf("Warning: could not watch %s: %v", wsFile, err)
		return
	}
	log.Printf("Watching %s for changes", wsFile)

	go func() {
		defer watcher.Close()
		var debounce <-chan time.Time
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Debounce: wait 500 ms for rapid successive writes to settle
					debounce = time.After(500 * time.Millisecond)
				}
			case <-debounce:
				log.Println("workspaces.yaml changed — triggering hot-reload")
				if d.monitor != nil {
					d.monitor.ReloadWorkspaces()
					go func() {
						if err := trigger.NewHTTPTriggerClient().SendWorkspaceReload(); err != nil {
							log.Printf("Could not notify Python of workspace reload: %v", err)
						}
					}()
				}
				debounce = nil
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Workspace file watcher error: %v", err)
			case <-d.ctx.Done():
				return
			}
		}
	}()
}

// KillDaemon forcefully kills a running daemon process
func KillDaemon(pidFile string) error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return fmt.Errorf("invalid PID in file: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	// Send stop signal (SIGTERM on Unix, TerminateProcess on Windows)
	if err := sendStopSignal(process); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Wait for process to exit (with timeout)
	for i := 0; i < 10; i++ {
		if !CheckProcessAlive(pid) {
			os.Remove(pidFile)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Force kill if still running
	log.Println("Process did not exit gracefully, sending SIGKILL...")
	if err := process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	os.Remove(pidFile)
	return nil
}
