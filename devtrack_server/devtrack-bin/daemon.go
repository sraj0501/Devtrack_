package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Daemon manages the background process lifecycle
type Daemon struct {
	monitor      *IntegratedMonitor
	config       *Config
	pidFile      string
	logFile      string
	ctx          context.Context
	cancel       context.CancelFunc
	isRunning    bool
	pythonBridge *exec.Cmd
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
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := os.MkdirAll(GetPIDDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create PID directory: %w", err)
	}

	if err := os.MkdirAll(GetLogDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	daemon := &Daemon{
		config:  config,
		pidFile: GetPIDFilePath(),
		logFile: GetLogFilePath(),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Create integrated monitor
	monitor, err := NewIntegratedMonitor(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitor: %w", err)
	}

	daemon.monitor = monitor

	return daemon, nil
}

// Start starts the daemon process
func (d *Daemon) Start() error {
	// Check if already running
	if d.IsRunning() {
		pid, _ := d.readPID()
		return fmt.Errorf("daemon already running (PID: %d)", pid)
	}

	// Setup logging
	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	log.Println("Starting DevTrack daemon...")
	log.Printf("PID file: %s", d.pidFile)
	log.Printf("Log file: %s", d.logFile)
	log.Printf("Config: %s", GetConfigPath())

	// Write PID file
	if err := d.writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Start integrated monitoring
	if err := d.monitor.Start(); err != nil {
		d.cleanup()
		return fmt.Errorf("failed to start monitoring: %w", err)
	}

	// Start Python bridge AFTER IPC server is ready
	if err := d.startPythonBridge(); err != nil {
		log.Printf("Warning: Failed to start Python bridge: %v", err)
		log.Println("IPC functionality will be limited")
	}

	d.isRunning = true
	log.Println("✓ Daemon started successfully")

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

	// Stop Python bridge
	if d.pythonBridge != nil {
		log.Println("Stopping Python bridge...")
		if err := d.pythonBridge.Process.Kill(); err != nil {
			log.Printf("Warning: error stopping Python bridge: %v", err)
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
		ConfigPath: GetConfigPath(),
		LogPath:    d.logFile,
		PIDPath:    d.pidFile,
	}

	// Check if running
	status.Running = d.IsRunning()

	if status.Running {
		// Read PID
		pid, err := d.readPID()
		if err == nil {
			status.PID = pid
		}

		// Get monitoring stats if available
		if d.monitor != nil && d.monitor.scheduler != nil {
			stats := d.monitor.scheduler.GetStats()
			if count, ok := stats["trigger_count"].(int); ok {
				status.TriggerCount = count
			}
			if lastTrigger, ok := stats["last_trigger"].(time.Time); ok {
				status.LastTrigger = lastTrigger
			}
		}

		// Calculate uptime from log file
		if info, err := os.Stat(d.logFile); err == nil {
			status.StartTime = info.ModTime()
			status.Uptime = time.Since(status.StartTime)
		}
	}

	return status, nil
}

// IsRunning checks if the daemon is currently running
func (d *Daemon) IsRunning() bool {
	pid, err := d.readPID()
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process is alive (doesn't actually send a signal)
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Pause pauses the scheduler (but keeps daemon running)
func (d *Daemon) Pause() error {
	if !d.IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	if d.monitor != nil && d.monitor.scheduler != nil {
		d.monitor.scheduler.Pause()
		log.Println("✓ Scheduler paused")
		return nil
	}

	return fmt.Errorf("scheduler not available")
}

// Resume resumes the scheduler
func (d *Daemon) Resume() error {
	if !d.IsRunning() {
		return fmt.Errorf("daemon is not running")
	}

	if d.monitor != nil && d.monitor.scheduler != nil {
		d.monitor.scheduler.Resume()
		log.Println("✓ Scheduler resumed")
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

// setupSignalHandlers sets up handlers for graceful shutdown and force-trigger
func (d *Daemon) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR2)

	go func() {
		for sig := range sigChan {
			log.Printf("Received signal: %v", sig)

			switch sig {
			case syscall.SIGUSR2:
				// Force immediate trigger (from devtrack force-trigger)
				if d.monitor != nil && d.monitor.scheduler != nil {
					log.Println("Force trigger requested via signal")
					d.monitor.scheduler.ForceImmediate()
				}

			case syscall.SIGHUP:
				// Reload configuration
				log.Println("Reloading configuration...")
				if config, err := LoadConfig(); err == nil {
					d.config = config
					log.Println("✓ Configuration reloaded")
				} else {
					log.Printf("Error reloading config: %v", err)
				}

			case os.Interrupt, syscall.SIGTERM:
				// Graceful shutdown
				log.Println("Initiating graceful shutdown...")
				d.cancel()
				return
			}
		}
	}()
}

// writePID writes the current process ID to the PID file
func (d *Daemon) writePID() error {
	pid := os.Getpid()
	return os.WriteFile(d.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// readPID reads the PID from the PID file
func (d *Daemon) readPID() (int, error) {
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

// cleanup removes PID file and performs cleanup
func (d *Daemon) cleanup() {
	if err := os.Remove(d.pidFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove PID file: %v", err)
	}
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

// startPythonBridge starts the Python bridge process for IPC communication
func (d *Daemon) startPythonBridge() error {
	// Get python_bridge.py path from centralized config (fails if not found)
	bridgePath := GetPythonBridgePath()

	// Start Python bridge using uv run to access project dependencies
	log.Printf("Starting Python bridge: %s", bridgePath)

	var cmd *exec.Cmd
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot != "" {
		// Use uv run to execute with project dependencies
		cmd = exec.Command("uv", "run", "--directory", projectRoot, "python", bridgePath)
		cmd.Dir = projectRoot
		log.Printf("Using uv environment from: %s", projectRoot)
	} else {
		// Fallback to system python3 if PROJECT_ROOT not set
		cmd = exec.Command("python3", bridgePath)
		log.Printf("Warning: PROJECT_ROOT not set, using system python3")
	}

	// Redirect output to daemon log
	logFile, err := os.OpenFile(d.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Python bridge: %w", err)
	}

	d.pythonBridge = cmd
	log.Printf("✓ Python bridge started (PID: %d)", cmd.Process.Pid)

	// Give it a moment to connect to IPC server
	time.Sleep(time.Second)

	return nil
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

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for process to exit (with timeout)
	for i := 0; i < 10; i++ {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process has exited
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
