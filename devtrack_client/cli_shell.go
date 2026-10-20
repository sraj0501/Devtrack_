package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// handleShellInit outputs shell integration code appropriate for the requested shell.
// Bash/zsh (default): eval "$(devtrack shell-init)"
// PowerShell:         devtrack shell-init --powershell | Out-String | Invoke-Expression
func (cli *CLI) handleShellInit() error {
	args := os.Args[2:] // everything after "shell-init"
	powershell := false
	for _, a := range args {
		if a == "--powershell" || a == "-ps" {
			powershell = true
		}
	}

	if powershell {
		return cli.handleShellInitPowerShell()
	}
	return cli.handleShellInitBash()
}

// handleShellInitBash outputs the bash/zsh compatibility shim. Normal Git
// commands, including add and commit, always go directly to Git; DevTrack
// observes completed commits through its daemon/hook path.
func (cli *CLI) handleShellInitBash() error {
	fmt.Print(`# DevTrack shell integration
# Keeps legacy history/messages aliases without intercepting normal Git work.
# Add to ~/.zshrc or ~/.bashrc:
#   eval "$(devtrack shell-init)"

git() {
  # Only intercept when inside a git repo
  if command git rev-parse --git-dir >/dev/null 2>&1; then
    # Honour an explicit bypass of the optional aliases.
    if [ "${GIT_NO_DEVTRACK:-}" = "1" ]; then
      command git "$@"
      return $?
    fi

    local _dt_enabled=""

    # Fast path: per-repo opt-in/out via git config (reads .git/config, no subprocess)
    # 'devtrack enable-git'  enables observation and the optional aliases
    # 'devtrack disable-git' disables the aliases (and observation on restart)
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
        history|messages)
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

// handleShellInitPowerShell outputs the PowerShell compatibility shim. Normal
// Git commands always execute through the native Git application.
// Usage: devtrack shell-init --powershell | Out-String | Invoke-Expression
// Or add to $PROFILE: devtrack shell-init --powershell | Out-String | Invoke-Expression
func (cli *CLI) handleShellInitPowerShell() error {
	fmt.Print(`# DevTrack shell integration for PowerShell
# Add to your PowerShell profile ($PROFILE):
#   devtrack shell-init --powershell | Out-String | Invoke-Expression
#
# To find your profile path: echo $PROFILE
# To edit it: notepad $PROFILE

function git {
    param([Parameter(ValueFromRemainingArguments)]$gitArgs)

    # Honour an explicit bypass of the optional aliases.
    if ($env:GIT_NO_DEVTRACK -eq "1") {
        & (Get-Command git -CommandType Application).Source @gitArgs
        return
    }

    # Only intercept inside a git repo
    $null = & (Get-Command git -CommandType Application).Source rev-parse --git-dir 2>$null
    if ($LASTEXITCODE -ne 0) {
        & (Get-Command git -CommandType Application).Source @gitArgs
        return
    }

    # Fast path: per-repo opt-in/out via git config
    $dtEnabled = & (Get-Command git -CommandType Application).Source config --local devtrack.enabled 2>$null

    # Explicit opt-out
    if ($dtEnabled -eq "false") {
        & (Get-Command git -CommandType Application).Source @gitArgs
        return
    }

    # Slow path: check workspaces.yaml
    if ([string]::IsNullOrEmpty($dtEnabled) -and (Get-Command devtrack -ErrorAction SilentlyContinue)) {
        devtrack is-workspace 2>$null
        if ($LASTEXITCODE -eq 0) { $dtEnabled = "true" }
    }

    if ($dtEnabled -eq "true" -and $gitArgs.Count -gt 0) {
        switch ($gitArgs[0]) {
            { $_ -in 'history','messages' } {
                devtrack git @gitArgs
                return
            }
        }
    }

    & (Get-Command git -CommandType Application).Source @gitArgs
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

	// Repository membership comes only from workspaces.yaml.
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

// handleEnableGit enables silent observation in the current repository.
func (cli *CLI) handleEnableGit() error {
	cmd := exec.Command("git", "config", "--local", "devtrack.enabled", "true")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git config: %v\nAre you inside a git repository?", err)
	}
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve repository path: %w", err)
	}
	if err := InstallPostCommitHook(repoPath); err != nil {
		return fmt.Errorf("install silent commit hook: %w", err)
	}
	fmt.Println("✓ DevTrack git integration enabled for this repo.")
	fmt.Println("  Normal Git commands remain unchanged; completed commits are observed silently.")
	fmt.Println()
	if runtime.GOOS == "windows" {
		fmt.Println("  Optional history aliases — add to your PowerShell profile ($PROFILE):")
		fmt.Println(`    devtrack shell-init --powershell | Out-String | Invoke-Expression`)
		fmt.Println()
		fmt.Println("  To find your profile path:  echo $PROFILE")
		fmt.Println("  To reload without restarting: . $PROFILE")
		fmt.Println()
		fmt.Println("  Using Git Bash instead? Add to ~/.bashrc:")
		fmt.Println(`    eval "$(devtrack shell-init)"`)
	} else {
		fmt.Println("  Optional history aliases — add to ~/.zshrc or ~/.bashrc if desired:")
		fmt.Println(`    eval "$(devtrack shell-init)"`)
		fmt.Println()
		fmt.Println("  If already set up, reload your shell function to pick up any updates:")
		fmt.Println(`    eval "$(devtrack shell-init)"`)
	}
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
