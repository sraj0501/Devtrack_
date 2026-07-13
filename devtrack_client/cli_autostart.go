package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// launchdPlistTemplatePath returns the path to the bundled plist template.
// It searches relative to the running binary (supports both the dev tree and
// an installed binary whose configs dir lives beside it).
func launchdPlistTemplatePath() (string, error) {
	// 1. Try PROJECT_ROOT env var (set by daemon, set in .env, or by user)
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		// 2. Walk up from binary looking for Data/configs/dev.devtrack.plist
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		dir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(dir, "Data", "configs", "dev.devtrack.plist")
			if _, err := os.Stat(candidate); err == nil {
				// dir is the project root
				projectRoot = dir
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if projectRoot == "" {
		return "", fmt.Errorf("cannot determine project root: set PROJECT_ROOT or run from the DevTrack directory")
	}
	tmplPath := filepath.Join(projectRoot, "Data", "configs", "dev.devtrack.plist")
	if _, err := os.Stat(tmplPath); err != nil {
		return "", fmt.Errorf("plist template not found at %s: %w", tmplPath, err)
	}
	return tmplPath, nil
}

// launchdEnvVarPrefixes lists the env var prefixes that should be captured into
// the launchd plist. launchd starts processes with a minimal environment, so
// all vars devtrack needs must be embedded at install time.
var launchdEnvVarPrefixes = []string{
	"PROJECT_ROOT", "DEVTRACK_", "DATA_", "DATABASE_", "LOG_", "PID_",
	"CONFIG_", "LEARNING_", "WORKSPACES_", "CLI_",
	"IPC_", "WEBHOOK_",
	"LLM_", "OLLAMA_", "LMSTUDIO_", "OPENAI_", "ANTHROPIC_", "GROQ_",
	"GIT_SAGE_", "HTTP_", "PROMPT_",
	"AZURE_", "GITHUB_", "GITLAB_", "JIRA_", "TEAMS_",
	"EMAIL_", "TELEGRAM_", "SLACK_",
	"ALERT_", "ADMIN_",
	"WORK_", "TIMEZONE", "PROMPT_INTERVAL", "AUTO_SYNC", "OUTPUT_TYPE",
	"DAILY_REPORT_TIME", "WEEKLY_REPORT_DAY", "SEND_ON_TRIGGER", "SEND_DAILY_SUMMARY",
	"PYTHON_BRIDGE_SCRIPT",
	"NEWPROJECT_", "SPEC_", "VACATION_", "EOD_", "WORK_SESSION_",
	"SENTIMENT_", "IPC_RETRY_", "LLM_REQUEST_", "QUEUE_", "DEFERRED_",
	"HEALTH_",
}

// shouldCaptureForLaunchd returns true if the env var name should be included
// in the launchd plist's EnvironmentVariables.
func shouldCaptureForLaunchd(name string) bool {
	// Always include PATH and HOME so child processes work correctly.
	if name == "PATH" || name == "HOME" || name == "USER" || name == "SHELL" {
		return true
	}
	for _, prefix := range launchdEnvVarPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
		if name == strings.TrimRight(prefix, "_") {
			return true
		}
	}
	return false
}

// xmlEscape escapes special XML characters in an attribute/text value.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// resolveAutostartLogDir returns the directory to write launchd/systemd
// wrapper logs into. Prefers LOG_DIR (set by 'devtrack setup', always a real
// writable directory under the XDG data home) over <projectRoot>/Data/logs:
// PROJECT_ROOT now points at the cloned Python server directory (managed
// install), which has no Data/ subdirectory of its own — only the pre-split
// dev-tree layout did.
func resolveAutostartLogDir(projectRoot string) string {
	if logDir := os.Getenv("LOG_DIR"); logDir != "" {
		return logDir
	}
	return filepath.Join(projectRoot, "Data", "logs")
}

// buildLaunchdPlist generates a launchd plist that embeds all current devtrack
// env vars so launchd can start the daemon with the correct environment.
func buildLaunchdPlist(binaryPath, projectRoot string) string {
	// Collect env vars to inject.
	type envEntry struct{ key, val string }
	var entries []envEntry
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if shouldCaptureForLaunchd(key) {
			entries = append(entries, envEntry{key, val})
		}
	}
	// Sort for deterministic output.
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].key > entries[j].key {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var envXML strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&envXML, "        <key>%s</key>\n        <string>%s</string>\n",
			xmlEscape(e.key), xmlEscape(e.val))
	}

	logPath := filepath.Join(resolveAutostartLogDir(projectRoot), "launchd.log")

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.devtrack</string>

    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
%s    </dict>

    <key>WorkingDirectory</key>
    <string>%s</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <false/>

    <key>StandardOutPath</key>
    <string>%s</string>

    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, xmlEscape(binaryPath), envXML.String(), xmlEscape(projectRoot),
		xmlEscape(logPath), xmlEscape(logPath))
}

// handleLaunchdInstall installs the launchd plist to ~/Library/LaunchAgents
// and loads it with launchctl so DevTrack auto-starts on login.
// All current devtrack env vars are baked into the plist so launchd can start
// the daemon with the correct environment (environment-first config).
func (cli *CLI) handleLaunchdInstall() error {
	// Resolve project root (used as WorkingDirectory and in env paths)
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		config, _ := LoadEnvConfig()
		if config != nil {
			projectRoot = config.ProjectRoot
		}
	}
	if projectRoot == "" {
		// Derive from the binary location
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		dir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(dir, "Data", "configs")); err == nil {
				projectRoot = dir
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if projectRoot == "" {
		return fmt.Errorf("cannot determine PROJECT_ROOT; export it or run from the DevTrack directory")
	}
	projectRoot, _ = filepath.Abs(projectRoot)

	// Resolve the DevTrack binary path
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = os.Args[0]
	}
	binaryPath, _ = filepath.Abs(binaryPath)

	// Ensure the log directory exists before launchd tries to redirect
	// stdout/stderr into it — launchd fails silently (or refuses to start
	// the job) if StandardOutPath's parent directory is missing.
	if err := os.MkdirAll(resolveAutostartLogDir(projectRoot), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Generate plist content with current env vars embedded.
	plistContent := buildLaunchdPlist(binaryPath, projectRoot)

	// Determine destination: ~/Library/LaunchAgents/dev.devtrack.plist
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}
	destPath := filepath.Join(launchAgentsDir, "dev.devtrack.plist")

	// Unload first if already loaded (ignore errors — it may not be loaded yet)
	_ = exec.Command("launchctl", "unload", destPath).Run()

	// Write the plist
	if err := os.WriteFile(destPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist to %s: %w", destPath, err)
	}

	// Load with launchctl
	loadCmd := exec.Command("launchctl", "load", destPath)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("launchctl load failed: %w\nPlist installed at %s — load it manually with: launchctl load %s", err, destPath, destPath)
	}

	fmt.Println("DevTrack launchd service installed.")
	fmt.Printf("  Plist:   %s\n", destPath)
	fmt.Printf("  Binary:  %s\n", binaryPath)
	fmt.Printf("  Root:    %s\n", projectRoot)
	fmt.Println()
	fmt.Println("DevTrack will now start automatically at login.")
	fmt.Println("Tip: re-run 'devtrack autostart-install' after changing env vars.")
	fmt.Println("Use 'devtrack status' to verify it is running.")
	fmt.Println("Use 'devtrack launchd-uninstall' to remove auto-start.")
	return nil
}

// handleLaunchdUninstall unloads and removes the launchd plist.
func (cli *CLI) handleLaunchdUninstall() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "dev.devtrack.plist")

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Println("DevTrack launchd service is not installed.")
		return nil
	}

	// Unload
	unloadCmd := exec.Command("launchctl", "unload", plistPath)
	unloadCmd.Stdout = os.Stdout
	unloadCmd.Stderr = os.Stderr
	if err := unloadCmd.Run(); err != nil {
		fmt.Printf("Warning: launchctl unload returned an error: %v\n", err)
		fmt.Println("Proceeding to remove plist anyway...")
	}

	// Remove plist
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("failed to remove plist %s: %w", plistPath, err)
	}

	fmt.Println("DevTrack launchd service removed.")
	fmt.Printf("  Removed: %s\n", plistPath)
	fmt.Println()
	fmt.Println("DevTrack will no longer start automatically at login.")
	fmt.Println("The running daemon (if any) was not stopped — use 'devtrack stop' to stop it.")
	return nil
}

// ---------------------------------------------------------------------------
// OS-agnostic autostart (detectOSType / handleAutostart*)
// ---------------------------------------------------------------------------

// osType enumerates the autostart mechanisms we support.
type osType string

const (
	osDarwin       osType = "darwin"
	osLinuxSystemd osType = "linux-systemd"
	osWSLSystemd   osType = "wsl-systemd"
	osWSLNoSystemd osType = "wsl-nosystemd"
	osWindows      osType = "windows"
)

// detectOSType returns the appropriate autostart mechanism for the current OS.
func detectOSType() osType {
	switch runtime.GOOS {
	case "darwin":
		return osDarwin
	case "linux":
		if isWSL() {
			if hasSystemd() {
				return osWSLSystemd
			}
			return osWSLNoSystemd
		}
		if hasSystemd() {
			return osLinuxSystemd
		}
		// Linux without systemd — fall back to profile-based (same as WSL-nosystemd)
		return osWSLNoSystemd
	case "windows":
		return osWindows
	default:
		return osDarwin // best guess for unknown OS
	}
}

// isWSL returns true when running inside Windows Subsystem for Linux.
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// hasSystemd returns true when systemd is the active init system.
func hasSystemd() bool {
	// Most reliable: private systemd runtime directory.
	if _, err := os.Stat("/run/systemd/private"); err == nil {
		return true
	}
	// Check PID-1 comm.
	if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		if strings.TrimSpace(string(data)) == "systemd" {
			return true
		}
	}
	// Fall back to pidof.
	if err := exec.Command("pidof", "systemd").Run(); err == nil {
		return true
	}
	return false
}

// resolveProjectRootForAutostart returns the project root, trying PROJECT_ROOT
// env var first and then walking up from the binary.
func resolveProjectRootForAutostart() (string, error) {
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		config, _ := LoadEnvConfig()
		if config != nil {
			projectRoot = config.ProjectRoot
		}
	}
	if projectRoot == "" {
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		dir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(dir, "Data", "configs")); err == nil {
				projectRoot = dir
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if projectRoot == "" {
		return "", fmt.Errorf("cannot determine PROJECT_ROOT; set it in .env or as an environment variable")
	}
	abs, _ := filepath.Abs(projectRoot)
	return abs, nil
}

// buildSystemdService generates a systemd user unit file that embeds all current
// devtrack env vars as Environment= lines.
func buildSystemdService(binaryPath, projectRoot string) string {
	var envLines strings.Builder
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if shouldCaptureForLaunchd(key) {
			// systemd requires quoting values that contain special chars.
			// Use double-quotes; escape any existing double-quotes.
			escaped := strings.ReplaceAll(val, `"`, `\"`)
			fmt.Fprintf(&envLines, "Environment=\"%s=%s\"\n", key, escaped)
		}
	}

	logPath := filepath.Join(resolveAutostartLogDir(projectRoot), "systemd.log")

	return fmt.Sprintf(`[Unit]
Description=DevTrack Developer Automation
After=default.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=%s start
%sWorkingDirectory=%s
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, binaryPath, envLines.String(), projectRoot, logPath, logPath)
}

// installSystemdService writes the unit file and enables + starts it.
// All current devtrack env vars are embedded as Environment= lines so systemd
// starts the daemon with the correct environment (environment-first config).
func installSystemdService(projectRoot, binaryPath, _ string) error {
	// Ensure the log directory exists before systemd tries to append to it —
	// StandardOutput=append: requires the parent directory to already exist,
	// otherwise the unit fails to start.
	if err := os.MkdirAll(resolveAutostartLogDir(projectRoot), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	svcContent := buildSystemdService(binaryPath, projectRoot)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	svcDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}
	destPath := filepath.Join(svcDir, "devtrack.service")
	if err := os.WriteFile(destPath, []byte(svcContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	for _, args := range [][]string{
		{"--user", "daemon-reload"},
		{"--user", "enable", "devtrack"},
		{"--user", "start", "devtrack"},
	} {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %s failed: %w", strings.Join(args, " "), err)
		}
	}

	fmt.Println("DevTrack systemd user service installed.")
	fmt.Printf("  Unit:    %s\n", destPath)
	fmt.Printf("  Binary:  %s\n", binaryPath)
	fmt.Printf("  Root:    %s\n", projectRoot)
	fmt.Println()
	fmt.Println("DevTrack will now start automatically at login.")
	fmt.Println("Tip: re-run 'devtrack autostart-install' after changing env vars.")
	fmt.Println("Use 'devtrack status' to verify it is running.")
	fmt.Println("Use 'devtrack autostart-uninstall' to remove auto-start.")
	return nil
}

// uninstallSystemdService stops, disables, and removes the unit file.
func uninstallSystemdService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	svcPath := filepath.Join(homeDir, ".config", "systemd", "user", "devtrack.service")

	if _, err := os.Stat(svcPath); os.IsNotExist(err) {
		fmt.Println("DevTrack systemd user service is not installed.")
		return nil
	}

	for _, args := range [][]string{
		{"--user", "stop", "devtrack"},
		{"--user", "disable", "devtrack"},
	} {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: systemctl %s returned an error: %v\n", strings.Join(args, " "), err)
		}
	}

	if err := os.Remove(svcPath); err != nil {
		return fmt.Errorf("failed to remove service file %s: %w", svcPath, err)
	}

	// Reload so systemd forgets the unit.
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	fmt.Println("DevTrack systemd user service removed.")
	fmt.Printf("  Removed: %s\n", svcPath)
	fmt.Println()
	fmt.Println("DevTrack will no longer start automatically at login.")
	fmt.Println("The running daemon (if any) was not stopped — use 'devtrack stop' to stop it.")
	return nil
}

// profileShellFile returns the preferred shell profile path for the current
// user (~/.zshrc if it exists, otherwise ~/.bashrc).
func profileShellFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	zshrc := filepath.Join(homeDir, ".zshrc")
	if _, err := os.Stat(zshrc); err == nil {
		return zshrc, nil
	}
	return filepath.Join(homeDir, ".bashrc"), nil
}

const (
	autostartMarkerBegin = "# DevTrack auto-start"
	autostartMarkerEnd   = "# End DevTrack auto-start"
)

// installProfileAutostart appends a startup block to the shell profile.
// The block simply calls 'devtrack start'; env vars must already be set in
// the shell profile (via direnv, manual export, etc.) before this line runs.
func installProfileAutostart(binaryPath, _ string) error {
	profilePath, err := profileShellFile()
	if err != nil {
		return err
	}

	// Read existing profile to check for idempotency.
	existing := ""
	if data, err := os.ReadFile(profilePath); err == nil {
		existing = string(data)
	}
	if strings.Contains(existing, autostartMarkerBegin) {
		fmt.Printf("DevTrack auto-start block already present in %s\n", profilePath)
		fmt.Println("Use 'devtrack autostart-uninstall' first if you want to reinstall.")
		return nil
	}

	block := fmt.Sprintf("\n%s\n%s start 2>/dev/null || true\n%s\n",
		autostartMarkerBegin, binaryPath, autostartMarkerEnd)

	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", profilePath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		return fmt.Errorf("failed to write to %s: %w", profilePath, err)
	}

	fmt.Println("DevTrack auto-start block added.")
	fmt.Printf("  Profile: %s\n", profilePath)
	fmt.Printf("  Binary:  %s\n", binaryPath)
	fmt.Println()
	fmt.Println("DevTrack will start automatically when a new shell session opens.")
	fmt.Printf("Re-source now:  source %s\n", profilePath)
	fmt.Println("Use 'devtrack autostart-uninstall' to remove auto-start.")
	return nil
}

// uninstallProfileAutostart removes the startup block from the shell profile.
func uninstallProfileAutostart() error {
	profilePath, err := profileShellFile()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No shell profile found — nothing to remove.")
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", profilePath, err)
	}

	content := string(data)
	if !strings.Contains(content, autostartMarkerBegin) {
		fmt.Printf("DevTrack auto-start block not found in %s\n", profilePath)
		return nil
	}

	// Remove the block between markers (inclusive).
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	inBlock := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, autostartMarkerBegin) {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.Contains(line, autostartMarkerEnd) {
				inBlock = false
			}
			continue
		}
		lines = append(lines, line)
	}

	newContent := strings.Join(lines, "\n")
	// Trim trailing blank lines added by the removal.
	newContent = strings.TrimRight(newContent, "\n") + "\n"

	if err := os.WriteFile(profilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", profilePath, err)
	}

	fmt.Println("DevTrack auto-start block removed.")
	fmt.Printf("  Profile: %s\n", profilePath)
	fmt.Println()
	fmt.Println("DevTrack will no longer start automatically on new shell sessions.")
	return nil
}

// handleAutostartInstall installs the OS-appropriate auto-start mechanism.
// All required env vars are captured from the current environment and baked
// into the service file / plist at install time.
func (cli *CLI) handleAutostartInstall() error {
	projectRoot, err := resolveProjectRootForAutostart()
	if err != nil {
		return err
	}

	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = os.Args[0]
	}
	binaryPath, _ = filepath.Abs(binaryPath)

	switch detectOSType() {
	case osDarwin:
		return cli.handleLaunchdInstall()
	case osLinuxSystemd, osWSLSystemd:
		return installSystemdService(projectRoot, binaryPath, "")
	case osWSLNoSystemd:
		return installProfileAutostart(binaryPath, "")
	case osWindows:
		return installWindowsTask(binaryPath)
	default:
		return cli.handleLaunchdInstall()
	}
}

// handleAutostartUninstall removes the OS-appropriate auto-start mechanism.
func (cli *CLI) handleAutostartUninstall() error {
	switch detectOSType() {
	case osDarwin:
		return cli.handleLaunchdUninstall()
	case osLinuxSystemd, osWSLSystemd:
		return uninstallSystemdService()
	case osWSLNoSystemd:
		return uninstallProfileAutostart()
	case osWindows:
		return removeWindowsScheduledTask()
	default:
		return cli.handleLaunchdUninstall()
	}
}

// handleAutostartStatus shows the status of the OS-appropriate auto-start mechanism.
func (cli *CLI) handleAutostartStatus() error {
	ot := detectOSType()
	switch ot {
	case osDarwin:
		fmt.Println("Auto-start mechanism: launchd (macOS)")
		fmt.Println()
		cmd := exec.Command("launchctl", "list")
		out, err := cmd.Output()
		if err != nil {
			fmt.Println("launchctl list failed — launchd may not be available.")
		} else {
			found := false
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "dev.devtrack") {
					fmt.Printf("  %s\n", strings.TrimSpace(line))
					found = true
				}
			}
			if !found {
				fmt.Println("  Service not registered (devtrack autostart-install to add it).")
			}
		}

	case osLinuxSystemd, osWSLSystemd:
		label := "Linux"
		if ot == osWSLSystemd {
			label = "WSL"
		}
		fmt.Printf("Auto-start mechanism: systemd user service (%s)\n", label)
		fmt.Println()
		cmd := exec.Command("systemctl", "--user", "status", "devtrack")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // non-zero exit when stopped is normal

	case osWSLNoSystemd:
		fmt.Println("Auto-start mechanism: shell profile (WSL without systemd)")
		fmt.Println()
		profilePath, err := profileShellFile()
		if err != nil {
			return err
		}
		data, _ := os.ReadFile(profilePath)
		if strings.Contains(string(data), autostartMarkerBegin) {
			fmt.Printf("  Block present in: %s\n", profilePath)
		} else {
			fmt.Printf("  Block NOT present in: %s\n", profilePath)
			fmt.Println("  Run 'devtrack autostart-install' to add it.")
		}
		fmt.Println()
		// Also show daemon status.
		return cli.handleStatus()

	case osWindows:
		fmt.Println("Auto-start mechanism: Windows Task Scheduler")
		fmt.Println()
		cmd := exec.Command("schtasks", "/Query", "/TN", "DevTrack", "/FO", "LIST")
		out, err := cmd.CombinedOutput()
		if err != nil {
			s := string(out)
			if strings.Contains(s, "does not exist") || strings.Contains(s, "not found") ||
				strings.Contains(strings.ToLower(s), "cannot find") {
				fmt.Println("  Task not registered (run 'devtrack autostart-install' to add it).")
			} else {
				fmt.Printf("  schtasks query failed: %s\n", strings.TrimSpace(s))
			}
		} else {
			fmt.Print(string(out))
		}
		fmt.Println()
	}

	return nil
}

// buildWindowsBat generates a Windows .bat file that sets all devtrack env vars
// and then launches the daemon. Mirrors buildLaunchdPlist / buildSystemdService.
// All values that contain % are escaped as %% so cmd.exe does not expand them.
func buildWindowsBat(binaryPath string) string {
	var b strings.Builder
	b.WriteString("@echo off\r\n")
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if !shouldCaptureForLaunchd(key) {
			continue
		}
		// Escape % so cmd.exe does not try to expand variable references.
		escaped := strings.ReplaceAll(val, "%", "%%")
		fmt.Fprintf(&b, "SET %s=%s\r\n", key, escaped)
	}
	fmt.Fprintf(&b, "%q start\r\n", binaryPath)
	return b.String()
}

// writeWindowsBat writes the devtrack-autostart.bat file next to the binary
// and returns its path. The bat file is used by schtasks as the task action
// so that env vars are set before the daemon starts.
func writeWindowsBat(binaryPath string) (string, error) {
	batPath := filepath.Join(filepath.Dir(binaryPath), "devtrack-autostart.bat")
	content := buildWindowsBat(binaryPath)
	if err := os.WriteFile(batPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write bat file %s: %w", batPath, err)
	}
	return batPath, nil
}

// installWindowsTask creates a Task Scheduler task that runs 'devtrack start' at logon.
// It writes a .bat file alongside the binary that bakes in all current devtrack env vars,
// then registers that bat file with schtasks (mirrors the launchd/systemd env-var pattern).
func installWindowsTask(binaryPath string) error {
	batPath, err := writeWindowsBat(binaryPath)
	if err != nil {
		return err
	}

	// Run the bat through cmd.exe /c so the SET lines are interpreted correctly.
	taskAction := fmt.Sprintf(`cmd.exe /c "%s"`, batPath)
	cmd := exec.Command("schtasks", "/Create", "/F",
		"/TN", "DevTrack",
		"/TR", taskAction,
		"/SC", "ONLOGON",
		"/RL", "HIGHEST",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create failed: %s", strings.TrimSpace(string(out)))
	}
	fmt.Println("DevTrack will now start automatically at logon.")
	fmt.Printf("  Bat:     %s\n", batPath)
	fmt.Printf("  Binary:  %s\n", binaryPath)
	fmt.Println()
	fmt.Println("Tip: re-run 'devtrack autostart-install' after changing env vars.")
	fmt.Println("Use 'devtrack status' to verify it is running.")
	fmt.Println("Use 'devtrack autostart-uninstall' to remove auto-start.")
	return nil
}
