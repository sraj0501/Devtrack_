package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// handleShellInit outputs shell integration code for eval "$(devtrack shell-init)"
// This defines a git() function that transparently routes commit/history/messages
// through DevTrack for monitored workspaces, passing everything else to real git.
func (cli *CLI) handleShellInit() error {
	fmt.Print(`# DevTrack shell integration
# Transparently routes git commands through DevTrack for monitored workspaces.
# Add to ~/.zshrc or ~/.bashrc:
#   eval "$(devtrack shell-init)"

git() {
  # Only intercept when inside a git repo
  if command git rev-parse --git-dir >/dev/null 2>&1; then
    # Honour explicit bypass: GIT_NO_DEVTRACK=1 git commit
    if [ "${GIT_NO_DEVTRACK:-}" = "1" ]; then
      command git "$@"
      return $?
    fi

    local _dt_enabled=""

    # Fast path: per-repo opt-in/out via git config (reads .git/config, no subprocess)
    # 'devtrack enable-git'  sets devtrack.enabled=true  → always intercept
    # 'devtrack disable-git' sets devtrack.enabled=false → never intercept (overrides workspaces.yaml)
    _dt_enabled=$(command git config --local devtrack.enabled 2>/dev/null || true)

    # Explicit opt-out: skip even if this repo is in workspaces.yaml
    if [ "$_dt_enabled" = "false" ]; then
      command git "$@"
      return $?
    fi

    # Slow path: check workspaces.yaml when not explicitly set
    if [ -z "$_dt_enabled" ] && command -v devtrack >/dev/null 2>&1; then
      if devtrack is-workspace 2>/dev/null; then
        _dt_enabled="true"
      fi
    fi

    if [ "$_dt_enabled" = "true" ]; then
      case "$1" in
        commit|history|messages|add)
          devtrack git "$@"
          return $?
          ;;
      esac
    fi
  fi

  command git "$@"
}
`)
	return nil
}

// handleIsWorkspace exits 0 if the current directory is a DevTrack workspace, 1 otherwise.
// Used by the shell-init git() function to decide whether to intercept git commands.
func (cli *CLI) handleIsWorkspace() error {
	// Get the git root of the current directory
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		os.Exit(1) // not a git repo
	}
	gitRoot := strings.TrimSpace(string(out))
	gitRoot, _ = filepath.Abs(gitRoot)

	// Single-repo mode: check DEVTRACK_WORKSPACE
	workspacePath := strings.TrimSpace(os.Getenv("DEVTRACK_WORKSPACE"))
	if workspacePath != "" {
		wsAbs, _ := filepath.Abs(workspacePath)
		if wsAbs == gitRoot || strings.HasPrefix(gitRoot, wsAbs+string(filepath.Separator)) {
			os.Exit(0)
		}
	}

	// Multi-repo mode: check workspaces.yaml
	wsCfg, err := LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		os.Exit(1)
	}
	for _, ws := range wsCfg.GetEnabledWorkspaces() {
		wsPath, _ := filepath.Abs(ws.Path)
		if wsPath == gitRoot || strings.HasPrefix(gitRoot, wsPath+string(filepath.Separator)) {
			os.Exit(0)
		}
	}

	os.Exit(1)
	return nil
}

// handleEnableGit sets git config devtrack.enabled=true in the current repo,
// opting it into DevTrack shell integration without editing workspaces.yaml.
func (cli *CLI) handleEnableGit() error {
	cmd := exec.Command("git", "config", "--local", "devtrack.enabled", "true")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git config: %v\nAre you inside a git repository?", err)
	}
	fmt.Println("✓ DevTrack git integration enabled for this repo.")
	fmt.Println("  git add, git commit, git history will now route through DevTrack.")
	fmt.Println()
	fmt.Println("  Shell integration required — add to ~/.zshrc or ~/.bashrc if not done yet:")
	fmt.Println(`    eval "$(devtrack shell-init)"`)
	fmt.Println()
	fmt.Println("  If already set up, reload your shell function to pick up any updates:")
	fmt.Println(`    eval "$(devtrack shell-init)"`)
	fmt.Println()
	fmt.Println("  To disable: devtrack disable-git")
	return nil
}

// handleDisableGit sets git config devtrack.enabled=false in the current repo.
// Setting false explicitly overrides workspaces.yaml detection in the shell function.
// (Simply unsetting the key would leave workspaces.yaml matching active.)
func (cli *CLI) handleDisableGit() error {
	cmd := exec.Command("git", "config", "--local", "devtrack.enabled", "false")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git config: %v\nAre you inside a git repository?", err)
	}
	fmt.Println("✓ DevTrack git integration disabled for this repo.")
	fmt.Println("  'git commit' will use standard git (even if this repo is in workspaces.yaml).")
	fmt.Println()
	fmt.Println("  To re-enable: devtrack enable-git")
	return nil
}

// enableGitForWorkspaces sets devtrack.enabled=true and installs the post-commit
// hook in all enabled workspaces. Called automatically on `devtrack start`.
func enableGitForWorkspaces() {
	cfg, err := LoadWorkspacesConfig()
	if err != nil || cfg == nil {
		return
	}
	for _, ws := range cfg.GetEnabledWorkspaces() {
		cmd := exec.Command("git", "-C", ws.Path, "config", "--local", "devtrack.enabled", "true")
		if err := cmd.Run(); err != nil {
			continue
		}
		if err := InstallPostCommitHook(ws.Path); err != nil {
			fmt.Printf("  ⚠ Git integration enabled for %s but hook install failed: %v\n", ws.Name, err)
		} else {
			_ = InstallPrePushHook(ws.Path) // best-effort; never blocks setup
			fmt.Printf("  ✓ Git integration enabled: %s\n", ws.Name)
		}
	}
}
