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

// RunUninstall implements `devtrack uninstall [--yes] [--keep-data]`.
// Removes all DevTrack components: daemon, autostart, shell integration, config, data, binary.
func RunUninstall(keepData, yes bool) error {
	fmt.Println("DevTrack Uninstall")
	fmt.Println("==================")
	fmt.Println()

	dataHome, err := devtrackDataHome()
	if err != nil {
		dataHome = "(unknown)"
	}
	home, _ := os.UserHomeDir()
	confDir := filepath.Join(home, ".devtrack")

	execPath, _ := os.Executable()
	if execPath != "" {
		execPath, _ = filepath.EvalSymlinks(execPath)
	}

	fmt.Println("The following will be removed:")
	fmt.Println("  • DevTrack daemon (if running)")
	fmt.Println("  • Autostart service")
	fmt.Println("  • Shell integration (profile lines)")
	fmt.Printf("  • %s  (config directory)\n", confDir)
	if !keepData {
		fmt.Printf("  • %s  (data, .env, workspaces.yaml, and the cloned Python server if managed mode installed one)\n", dataHome)
	} else {
		fmt.Printf("  Note: data directory kept: %s\n", dataHome)
	}
	if execPath != "" {
		fmt.Printf("  • %s  (binary)\n", execPath)
	}
	fmt.Println()

	if !yes {
		fmt.Print("Continue? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Uninstall cancelled.")
			return nil
		}
		fmt.Println()
	}

	// 1. Stop daemon
	fmt.Print("Stopping daemon... ")
	if isDaemonRunning() {
		if err := KillDaemon(GetPIDFilePath()); err != nil {
			fmt.Printf("warning: %v\n", err)
		} else {
			fmt.Println("done")
		}
	} else {
		fmt.Println("not running")
	}

	// 2. Remove autostart
	fmt.Print("Removing autostart... ")
	if err := removeAutostart(); err != nil {
		fmt.Printf("warning: %v\n", err)
	} else {
		fmt.Println("done")
	}

	// 3. Remove shell integration
	fmt.Println("Removing shell integration...")
	removeShellIntegration()

	// 4. Remove config directory
	fmt.Printf("Removing config directory %s... ", confDir)
	if err := os.RemoveAll(confDir); err != nil {
		fmt.Printf("warning: %v\n", err)
	} else {
		fmt.Println("done")
	}

	// 5. Remove data directory (unless --keep-data)
	if !keepData {
		fmt.Printf("Removing data directory %s... ", dataHome)
		if err := os.RemoveAll(dataHome); err != nil {
			fmt.Printf("warning: %v\n", err)
		} else {
			fmt.Println("done")
		}
	}

	// 6. Remove binary (best-effort, platform-specific)
	if execPath != "" {
		fmt.Printf("Removing binary %s... ", execPath)
		if err := removeSelfBinary(execPath); err != nil {
			fmt.Printf("warning: %v\n", err)
		} else {
			fmt.Println("done")
		}
	}

	fmt.Println()
	fmt.Println("DevTrack has been uninstalled.")
	if keepData {
		fmt.Printf("Your data remains at: %s\n", dataHome)
		fmt.Println("Delete it manually when no longer needed.")
	}
	return nil
}

// removeAutostart removes the OS-appropriate autostart mechanism.
func removeAutostart() error {
	switch runtime.GOOS {
	case "darwin":
		return removeLaunchdService()
	case "linux":
		switch detectOSType() {
		case osLinuxSystemd, osWSLSystemd:
			return uninstallSystemdService()
		default:
			return uninstallProfileAutostart()
		}
	case "windows":
		return removeWindowsScheduledTask()
	}
	return nil
}

// removeLaunchdService unloads and removes the macOS launchd plist.
func removeLaunchdService() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "dev.devtrack.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return nil // not installed
	}
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	return os.Remove(plistPath)
}

// removeWindowsScheduledTask removes the DevTrack Task Scheduler task (best-effort).
func removeWindowsScheduledTask() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", "DevTrack", "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := string(out)
		if strings.Contains(s, "does not exist") || strings.Contains(s, "not found") ||
			strings.Contains(strings.ToLower(s), "cannot find") {
			return nil // task was never installed
		}
		return fmt.Errorf("schtasks: %w", err)
	}
	return nil
}

// removeShellIntegration removes the DevTrack eval block from common RC files.
func removeShellIntegration() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	candidates := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
	}
	for _, rc := range candidates {
		if removeMarkedBlock(rc, "# DevTrack shell integration", "devtrack shell-init") {
			fmt.Printf("  ✓ Cleaned %s\n", rc)
		}
	}
}

// removeMarkedBlock removes lines from path that contain blockStart or lineMarker,
// along with any blank lines that immediately follow. Returns true if anything was removed.
func removeMarkedBlock(path, blockStart, lineMarker string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	var out []string
	changed := false
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], blockStart) || strings.Contains(lines[i], lineMarker) {
			changed = true
			continue
		}
		out = append(out, lines[i])
	}
	if !changed {
		return false
	}
	// Trim trailing blank lines introduced by removal
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	_ = os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0644)
	return true
}
