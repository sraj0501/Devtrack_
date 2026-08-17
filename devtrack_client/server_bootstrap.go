package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	bootstrapStateFile = "server-bootstrap.json"
	bootstrapLockFile  = "server-bootstrap.lock"
	bootstrapLogFile   = "server-bootstrap.log"
)

type serverBootstrapState struct {
	Status      string    `json:"status"`
	Step        string    `json:"step,omitempty"`
	Message     string    `json:"message,omitempty"`
	Error       string    `json:"error,omitempty"`
	PID         int       `json:"pid,omitempty"`
	ProjectRoot string    `json:"project_root"`
	Provider    string    `json:"provider,omitempty"`
	Model       string    `json:"model,omitempty"`
	LogPath     string    `json:"log_path"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type bootstrapCommandRunner func(dir, name string, args ...string) error

func executeBootstrapCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func bootstrapPaths(home string) (state, lock, log string) {
	logDir := filepath.Join(home, "data", "logs")
	return filepath.Join(home, bootstrapStateFile), filepath.Join(home, bootstrapLockFile), filepath.Join(logDir, bootstrapLogFile)
}

func readServerBootstrapState(home string) (*serverBootstrapState, error) {
	statePath, _, _ := bootstrapPaths(home)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	var state serverBootstrapState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode bootstrap state: %w", err)
	}
	return &state, nil
}

func writeServerBootstrapState(home string, state *serverBootstrapState) error {
	statePath, _, logPath := bootstrapPaths(home)
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}
	state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(statePath), ".server-bootstrap-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return atomicReplaceFile(tmpPath, statePath)
}

func acquireBootstrapLock(home string) (func(), error) {
	_, lockPath, _ := bootstrapPaths(home)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, err
	}
	create := func() (*os.File, error) {
		return os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	}
	file, err := create()
	if errors.Is(err, os.ErrExist) {
		data, readErr := os.ReadFile(lockPath)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if readErr == nil && parseErr == nil && CheckProcessAlive(pid) {
			return nil, fmt.Errorf("server bootstrap is already running (PID %d)", pid)
		}
		if removeErr := os.Remove(lockPath); removeErr != nil {
			return nil, fmt.Errorf("remove stale bootstrap lock: %w", removeErr)
		}
		file, err = create()
	}
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		file.Close()
		os.Remove(lockPath)
		return nil, err
	}
	if err := file.Close(); err != nil {
		os.Remove(lockPath)
		return nil, err
	}
	return func() { _ = os.Remove(lockPath) }, nil
}

func runServerBootstrap(home, projectRoot, provider, model string, runner bootstrapCommandRunner) error {
	release, err := acquireBootstrapLock(home)
	if err != nil {
		return err
	}
	defer release()

	_, _, logPath := bootstrapPaths(home)
	state := &serverBootstrapState{
		Status: "running", PID: os.Getpid(), ProjectRoot: projectRoot,
		Provider: provider, Model: model, LogPath: logPath, StartedAt: time.Now().UTC(),
	}
	update := func(step, message string) error {
		state.Step, state.Message, state.Error = step, message, ""
		return writeServerBootstrapState(home, state)
	}
	fail := func(step string, cause error) error {
		state.Status, state.Step, state.Error = "failed", step, cause.Error()
		state.Message = "Bootstrap stopped; Go-native features remain available."
		_ = writeServerBootstrapState(home, state)
		return cause
	}

	backendDir := filepath.Join(projectRoot, "backend")
	targetDir := filepath.Dir(projectRoot)
	managedCheckout := filepath.Clean(targetDir) == filepath.Clean(filepath.Join(home, "server"))
	_, backendErr := os.Stat(backendDir)
	if backendErr != nil || managedCheckout {
		if err := update("cloning", "Downloading the optional Python server"); err != nil {
			return err
		}
		if _, gitErr := os.Stat(filepath.Join(targetDir, ".git")); gitErr != nil {
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fail("cloning", err)
			}
			if err := runner("", "git", "init", targetDir); err != nil {
				return fail("cloning", fmt.Errorf("git init: %w", err))
			}
		}
		// Removing a missing remote is harmless and makes retries safe after a
		// process stopped between git init and remote configuration.
		_ = runner("", "git", "-C", targetDir, "remote", "remove", "origin")
		remoteArgs := []string{"-C", targetDir, "remote", "add", "origin", devtrackServerRepoURL}
		for _, args := range [][]string{
			remoteArgs,
			{"-C", targetDir, "sparse-checkout", "init", "--cone"},
			{"-C", targetDir, "sparse-checkout", "set", "devtrack_server"},
			{"-C", targetDir, "fetch", "--depth", "1", "origin", devtrackServerBranch},
			{"-C", targetDir, "checkout", "-B", devtrackServerBranch, "FETCH_HEAD"},
		} {
			if err := runner("", "git", args...); err != nil {
				return fail("cloning", fmt.Errorf("git %s: %w", strings.Join(args, " "), err))
			}
		}
	}

	if err := update("syncing", "Installing Python dependencies with uv"); err != nil {
		return err
	}
	if err := runner(projectRoot, "uv", "sync"); err != nil {
		return fail("syncing", fmt.Errorf("uv sync: %w", err))
	}

	if provider == "ollama" && model != "" {
		if err := update("pulling_model", "Pulling local Ollama model "+model); err != nil {
			return err
		}
		if err := runner("", "ollama", "pull", model); err != nil {
			return fail("pulling_model", fmt.Errorf("ollama pull %s: %w", model, err))
		}
	}

	state.Status, state.Step, state.Message, state.Error = "ready", "complete", "Optional AI server dependencies are ready.", ""
	state.CompletedAt = time.Now().UTC()
	return writeServerBootstrapState(home, state)
}

func startServerBootstrap(home, projectRoot, provider, model string) (bool, error) {
	if state, err := readServerBootstrapState(home); err == nil && (state.Status == "queued" || state.Status == "running") {
		if state.PID == 0 || CheckProcessAlive(state.PID) {
			return false, nil
		}
	}
	_, _, logPath := bootstrapPaths(home)
	queued := &serverBootstrapState{
		Status: "queued", Step: "starting", Message: "Background bootstrap is starting.",
		ProjectRoot: projectRoot, Provider: provider, Model: model, LogPath: logPath,
	}
	if err := writeServerBootstrapState(home, queued); err != nil {
		return false, err
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return false, err
	}
	defer logFile.Close()
	cmd := exec.Command(exe, "bootstrap-server", "--home", home, "--project-root", projectRoot, "--provider", provider, "--model", model)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = logFile, logFile, nil
	setSetsid(cmd)
	if err := cmd.Start(); err != nil {
		queued.Status, queued.Error = "failed", err.Error()
		queued.Message = "Could not start the background worker."
		_ = writeServerBootstrapState(home, queued)
		return false, err
	}
	if current, readErr := readServerBootstrapState(home); readErr == nil && current.Status == "queued" {
		current.PID = cmd.Process.Pid
		_ = writeServerBootstrapState(home, current)
	}
	_ = cmd.Process.Release()
	return true, nil
}

func runServerBootstrapCommand(args []string) error {
	values := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		values[args[i]] = args[i+1]
	}
	for _, key := range []string{"--home", "--project-root"} {
		if values[key] == "" {
			return fmt.Errorf("bootstrap-server requires %s", key)
		}
	}
	return runServerBootstrap(values["--home"], values["--project-root"], values["--provider"], values["--model"], executeBootstrapCommand)
}

func printBootstrapCapabilities(out io.Writer) {
	home, err := devtrackDataHome()
	if err != nil {
		fmt.Fprintf(out, "Capabilities: unable to locate bootstrap state: %v\n\n", err)
		return
	}
	fmt.Fprintln(out, "Capabilities:")
	fmt.Fprintln(out, "  Git monitoring / scheduling     ready (Go-native)")
	fmt.Fprintln(out, "  Local SQLite / MCP              ready (Go-native)")
	fmt.Fprintln(out, "  PM connectors / pending queue   ready when configured (Go-native)")
	if GetServerMode() == ServerModeExternal {
		fmt.Fprintln(out, "  AI server                       external (bootstrap not applicable)")
		fmt.Fprintln(out)
		return
	}
	state, err := readServerBootstrapState(home)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "  AI reports / voice / enrichment pending (run 'devtrack doctor --repair')")
		fmt.Fprintln(out)
		return
	}
	if err != nil {
		fmt.Fprintf(out, "  AI reports / voice / enrichment degraded (%v)\n\n", err)
		return
	}
	switch state.Status {
	case "ready":
		fmt.Fprintln(out, "  AI server dependencies           ready")
	case "queued", "running":
		fmt.Fprintf(out, "  AI reports / voice / enrichment installing (%s)\n", state.Step)
	case "failed":
		fmt.Fprintf(out, "  AI reports / voice / enrichment degraded (%s failed)\n", state.Step)
		fmt.Fprintf(out, "    Error: %s\n", state.Error)
		fmt.Fprintln(out, "    Retry: devtrack doctor --repair")
	default:
		fmt.Fprintf(out, "  AI reports / voice / enrichment %s\n", state.Status)
	}
	if state.LogPath != "" && state.Status != "ready" {
		fmt.Fprintf(out, "    Log:   %s\n", state.LogPath)
	}
	fmt.Fprintln(out)
}

func RunDoctor(repair bool) error {
	if repair {
		if GetServerMode() == ServerModeExternal {
			return fmt.Errorf("server bootstrap is not applicable in external mode")
		}
		home, err := devtrackDataHome()
		if err != nil {
			return err
		}
		state, stateErr := readServerBootstrapState(home)
		if stateErr != nil {
			projectRoot := GetProjectRootOptional()
			if projectRoot == "" {
				return fmt.Errorf("no managed bootstrap configuration found; run 'devtrack setup'")
			}
			_, _, logPath := bootstrapPaths(home)
			state = &serverBootstrapState{
				ProjectRoot: projectRoot, Provider: GetLLMProvider(), Model: GetOllamaModel(), LogPath: logPath,
			}
		}
		started, err := startServerBootstrap(home, state.ProjectRoot, state.Provider, state.Model)
		if err != nil {
			return err
		}
		if started {
			fmt.Println("Background server repair started.")
		} else {
			fmt.Println("Background server bootstrap is already running.")
		}
	}
	printStatusServer()
	printBootstrapCapabilities(os.Stdout)
	return nil
}
