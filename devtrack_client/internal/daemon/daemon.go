package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/alerts"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/health"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/infra"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/notify"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/telegram"
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
	alertPoller   *alerts.Poller   // native Go alert poller (Phase 2)
	telegramBot   *telegram.Bot    // interactive Telegram bot (Phase 3)
	startTime     time.Time
	healthMonitor *health.HealthMonitor

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

	// Start integrated monitoring
	if err := d.monitor.Start(); err != nil {
		d.cleanup()
		return fmt.Errorf("failed to start monitoring: %w", err)
	}

	// Watch workspaces.yaml for changes — triggers hot-reload automatically
	d.startWorkspacesFileWatcher()

	// Generate TLS cert before starting any Python subprocess
	if config.IsTLSEnabled() {
		if err := d.generateTLSCert(); err != nil {
			log.Printf("Warning: TLS cert generation failed (%v) — disabling TLS", err)
			// Force TLS off so subsequent HTTP client builds don't fail
			os.Setenv("DEVTRACK_TLS", "false")
		}
	}

	// Start webhook server (primary Python process in CS-1)
	if err := d.startWebhookServer(); err != nil {
		log.Printf("Warning: Failed to start webhook server: %v", err)
		log.Println("HTTP trigger functionality will be unavailable")
	} else {
		// Wait up to 10 s for the Python HTTP server to become healthy
		d.waitForPythonHTTP(10)
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

	// Stop webhook server
	if d.webhookServer != nil {
		log.Println("Stopping webhook server...")
		if err := d.webhookServer.Process.Kill(); err != nil {
			log.Printf("Warning: error stopping webhook server: %v", err)
		}
	}

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

// generateTLSCert generates a self-signed TLS certificate for the Go↔Python channel.
func (d *Daemon) generateTLSCert() error {
	certPath := config.GetTLSCertPath()
	keyPath := config.GetTLSKeyPath()
	log.Printf("Generating TLS cert: %s", certPath)
	if err := trigger.GenerateSelfSignedCert(certPath, keyPath); err != nil {
		return fmt.Errorf("TLS cert generation: %w", err)
	}
	// Expose paths so subprocesses pick them up
	os.Setenv("DEVTRACK_TLS_CERT", certPath)
	os.Setenv("DEVTRACK_TLS_KEY", keyPath)
	log.Println("✓ TLS cert generated")
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
		cmd = exec.Command("python3", "-m", "backend.webhook_server")
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
	if d.webhookServer != nil && d.webhookServer.Process != nil {
		d.webhookServer.Process.Kill()
		d.webhookServer.Process.Wait()
	}

	if err := d.startWebhookServer(); err != nil {
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
	poller.Start(d.ctx)
	d.alertPoller = poller
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
