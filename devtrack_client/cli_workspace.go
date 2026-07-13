package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// WorkspaceCommands handles workspace management commands
type WorkspaceCommands struct{}

// NewWorkspaceCommands creates a new WorkspaceCommands handler
func NewWorkspaceCommands() *WorkspaceCommands {
	return &WorkspaceCommands{}
}

// List prints all configured workspaces from workspaces.yaml
func (wc *WorkspaceCommands) List() error {
	cfg, err := LoadWorkspacesConfig()
	if err != nil {
		fmt.Printf("Failed to load workspaces.yaml: %v\n", err)
		return err
	}

	if cfg == nil || len(cfg.Workspaces) == 0 {
		fmt.Println("No workspaces.yaml found. Running in single-repo mode.")
		return nil
	}

	fmt.Printf("%-20s %-40s %-12s %s\n", "NAME", "PATH", "PLATFORM", "ENABLED")
	fmt.Println(strings.Repeat("-", 80))
	for _, ws := range cfg.Workspaces {
		enabled := "no"
		if ws.Enabled {
			enabled = "yes"
		}
		platform := ws.PMPlatform
		if platform == "" {
			platform = "(none)"
		}
		fmt.Printf("%-20s %-40s %-12s %s\n", ws.Name, ws.Path, platform, enabled)
	}

	return nil
}

// Add adds a new workspace entry to workspaces.yaml (creating it if needed)
func (wc *WorkspaceCommands) Add(name, path, pmPlatform string) error {
	path = expandWorkspacePath(path)

	if !IsGitRepository(path) {
		reader := bufio.NewReader(os.Stdin)
		if err := offerGitInit(path, reader); err != nil {
			return err
		}
	}

	cfg, err := LoadWorkspacesConfig()
	if err != nil {
		return fmt.Errorf("failed to load workspaces.yaml: %w", err)
	}

	if cfg == nil {
		cfg = &WorkspacesConfig{
			Version:    "1",
			Workspaces: []WorkspaceConfig{},
		}
	}

	for _, ws := range cfg.Workspaces {
		if ws.Name == name {
			return fmt.Errorf("workspace %q already exists", name)
		}
	}

	cfg.Workspaces = append(cfg.Workspaces, WorkspaceConfig{
		Name:       name,
		Path:       path,
		PMPlatform: pmPlatform,
		Enabled:    true,
	})

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save workspaces.yaml: %w", err)
	}

	fmt.Printf("Added workspace %q (%s)\n", name, path)
	wc.sendWorkspaceReload()
	return nil
}

// Remove removes a workspace by name from workspaces.yaml
func (wc *WorkspaceCommands) Remove(name string) error {
	cfg, err := LoadWorkspacesConfig()
	if err != nil {
		return fmt.Errorf("failed to load workspaces.yaml: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("workspaces.yaml not found")
	}

	found := false
	updated := cfg.Workspaces[:0]
	for _, ws := range cfg.Workspaces {
		if ws.Name == name {
			found = true
			continue
		}
		updated = append(updated, ws)
	}

	if !found {
		return fmt.Errorf("workspace %q not found", name)
	}

	cfg.Workspaces = updated

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save workspaces.yaml: %w", err)
	}

	fmt.Printf("Removed workspace %q\n", name)
	wc.sendWorkspaceReload()
	return nil
}

// Enable sets enabled=true for a workspace by name
func (wc *WorkspaceCommands) Enable(name string) error {
	return wc.setEnabled(name, true)
}

// Disable sets enabled=false for a workspace by name
func (wc *WorkspaceCommands) Disable(name string) error {
	return wc.setEnabled(name, false)
}

func (wc *WorkspaceCommands) setEnabled(name string, enabled bool) error {
	cfg, err := LoadWorkspacesConfig()
	if err != nil {
		return fmt.Errorf("failed to load workspaces.yaml: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("workspaces.yaml not found")
	}

	found := false
	for i := range cfg.Workspaces {
		if cfg.Workspaces[i].Name == name {
			cfg.Workspaces[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("workspace %q not found", name)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save workspaces.yaml: %w", err)
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	fmt.Printf("Workspace %q %s\n", name, state)
	wc.sendWorkspaceReload()
	return nil
}

// sendWorkspaceReload notifies the running daemon via SIGHUP (best-effort, not fatal).
// The SIGHUP handler in daemon.go reloads Go workspaces and POSTs to Python.
func (wc *WorkspaceCommands) sendWorkspaceReload() {
	pid, err := readDaemonPID()
	if err != nil {
		fmt.Println("(Daemon not running — changes will take effect on next start.)")
		return
	}
	if !CheckProcessAlive(pid) {
		fmt.Println("(Daemon not running — changes will take effect on next start.)")
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("(Daemon not running — changes will take effect on next start.)")
		return
	}
	if err := SendReloadSignal(proc); err != nil {
		fmt.Printf("(%v)\n", err)
		return
	}
	fmt.Println("Reload signal sent to daemon.")
}

// Reload sends SIGHUP to the running daemon, triggering a workspace config reload.
func (wc *WorkspaceCommands) Reload() error {
	pid, err := readDaemonPID()
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}
	if err := SendReloadSignal(proc); err != nil {
		return err
	}
	fmt.Println("Workspace reload signal sent to daemon.")
	return nil
}

// InstallHooks installs the post-commit hook in every enabled workspace repo.
// Idempotent — safe to run multiple times. Prints a result line per workspace.
func (wc *WorkspaceCommands) InstallHooks() error {
	cfg, err := LoadWorkspacesConfig()
	if err != nil {
		return fmt.Errorf("failed to load workspaces.yaml: %w", err)
	}
	if cfg == nil || len(cfg.GetEnabledWorkspaces()) == 0 {
		fmt.Println("No workspaces configured. Add repos with: devtrack workspace add <name> <path>")
		return nil
	}

	ok, failed := 0, 0
	for _, ws := range cfg.GetEnabledWorkspaces() {
		if err := InstallPostCommitHook(ws.Path); err != nil {
			fmt.Printf("  ✗ %-20s %s — %v\n", ws.Name, ws.Path, err)
			failed++
		} else {
			_ = InstallPrePushHook(ws.Path) // best-effort; never blocks setup
			fmt.Printf("  ✓ %-20s %s\n", ws.Name, ws.Path)
			ok++
		}
	}
	fmt.Printf("\n%d hook(s) installed", ok)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()
	return nil
}

// offerGitInit prompts the user to initialize path as a git repository when
// it is not already one. If accepted it runs git init + an initial commit so
// the directory is immediately usable as a DevTrack workspace.
// Returns nil if the user accepts and init succeeds, or an error otherwise
// (including when the user declines — the caller decides whether to continue).
//
// Takes the caller's *bufio.Reader rather than wrapping os.Stdin again: a
// second independent bufio.Reader over the same stdin can silently steal
// already-buffered input from the caller (or vice versa), misaligning every
// prompt that follows — this bit a scripted `devtrack setup` run where the
// workspace-path answer was swallowed here instead of reaching this prompt.
func offerGitInit(path string, reader *bufio.Reader) error {
	fmt.Printf("\n%s is not a git repository.\n", path)
	fmt.Print("Initialize it with git init and an initial commit? [Y/n]: ")

	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "" && answer != "y" && answer != "yes" {
		return fmt.Errorf("%s is not a git repository", path)
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", path, err)
	}

	run := func(args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = path
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	fmt.Println()
	if err := run("git", "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	// Create a .gitkeep so there is something to commit in an empty directory.
	keepFile := path + "/.gitkeep"
	if _, err := os.Stat(keepFile); os.IsNotExist(err) {
		// Only create .gitkeep if the directory is empty.
		entries, _ := os.ReadDir(path)
		hasFiles := false
		for _, e := range entries {
			if e.Name() != ".git" {
				hasFiles = true
				break
			}
		}
		if !hasFiles {
			_ = os.WriteFile(keepFile, nil, 0644)
		}
	}

	if err := run("git", "add", "."); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	if err := run("git", "commit", "-m", "Initial commit"); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("Initialized git repository at %s\n", path)
	return nil
}

// readDaemonPID reads the daemon PID from the PID file.
func readDaemonPID() (int, error) {
	data, err := os.ReadFile(GetPIDFilePath())
	if err != nil {
		return 0, err
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, fmt.Errorf("invalid PID: %w", err)
	}
	return pid, nil
}
