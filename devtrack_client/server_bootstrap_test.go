package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerBootstrapFreshManagedCheckoutRunsSparseClone(t *testing.T) {
	home := t.TempDir()
	projectRoot := filepath.Join(home, "server", "devtrack_server")
	var calls []string
	runner := func(dir, name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	if err := runServerBootstrap(home, projectRoot, "openai", "", runner); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"git init " + filepath.Join(home, "server"),
		"git -C " + filepath.Join(home, "server") + " sparse-checkout set devtrack_server",
		"git -C " + filepath.Join(home, "server") + " checkout -B main FETCH_HEAD",
		"uv sync --extra ai",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("bootstrap commands missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "ollama pull") {
		t.Fatalf("cloud-provider bootstrap unexpectedly pulled an Ollama model:\n%s", joined)
	}
}

func TestServerBootstrapRejectsDuplicateWorker(t *testing.T) {
	home := t.TempDir()
	_, lockPath, _ := bootstrapPaths(home)
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	err := runServerBootstrap(home, filepath.Join(home, "server", "devtrack_server"), "ollama", "llama3.2", func(string, string, ...string) error {
		t.Fatal("duplicate bootstrap invoked a command")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("duplicate bootstrap error = %v", err)
	}
}

func TestServerBootstrapExistingCheckoutSyncsAndPullsLocalModel(t *testing.T) {
	home := t.TempDir()
	projectRoot := filepath.Join(t.TempDir(), "devtrack_server")
	if err := os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	runner := func(dir, name string, args ...string) error {
		calls = append(calls, dir+"|"+name+" "+strings.Join(args, " "))
		return nil
	}
	if err := runServerBootstrap(home, projectRoot, "ollama", "llama3.2", runner); err != nil {
		t.Fatal(err)
	}
	want := []string{
		projectRoot + "|uv sync --extra ai",
		"|ollama pull llama3.2",
		"|ollama pull nomic-embed-text",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %#v, want %#v", calls, want)
	}
	state, err := readServerBootstrapState(home)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "ready" || state.Step != "complete" || state.CompletedAt.IsZero() {
		t.Fatalf("unexpected completed state: %+v", state)
	}
}

func TestServerBootstrapSkipsPullForDetectedLocalModel(t *testing.T) {
	home := t.TempDir()
	projectRoot := filepath.Join(t.TempDir(), "devtrack_server")
	if err := os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	runner := func(dir, name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	if err := runServerBootstrap(home, projectRoot, "ollama", "qwen2.5:7b", runner, true); err != nil {
		t.Fatal(err)
	}
	want := "uv sync --extra ai\nollama pull nomic-embed-text"
	if got := strings.Join(calls, "\n"); got != want {
		t.Fatalf("bootstrap commands = %q, want %q", got, want)
	}
	state, err := readServerBootstrapState(home)
	if err != nil {
		t.Fatal(err)
	}
	if state.Model != "qwen2.5:7b" || !state.SkipModelPull {
		t.Fatalf("detected-model state not preserved: %+v", state)
	}
}

func TestServerBootstrapCloudProviderNeverPullsModel(t *testing.T) {
	home := t.TempDir()
	projectRoot := filepath.Join(t.TempDir(), "devtrack_server")
	if err := os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	var calls []string
	runner := func(dir, name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	if err := runServerBootstrap(home, projectRoot, "openai", "cloud-model", runner); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(calls, "\n"); got != "uv sync --extra ai" {
		t.Fatalf("cloud bootstrap commands = %q, want only uv sync --extra ai", got)
	}
}

func TestServerBootstrapFailureIsDurableAndNonBlocking(t *testing.T) {
	home := t.TempDir()
	projectRoot := filepath.Join(t.TempDir(), "devtrack_server")
	if err := os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	runner := func(dir, name string, args ...string) error {
		return errors.New("uv unavailable")
	}
	if err := runServerBootstrap(home, projectRoot, "ollama", "llama3.2", runner); err == nil {
		t.Fatal("expected bootstrap error")
	}
	state, err := readServerBootstrapState(home)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "failed" || state.Step != "syncing" || !strings.Contains(state.Error, "uv unavailable") {
		t.Fatalf("unexpected failure state: %+v", state)
	}
	var out bytes.Buffer
	oldHome := os.Getenv("XDG_DATA_HOME")
	t.Cleanup(func() { _ = os.Setenv("XDG_DATA_HOME", oldHome) })
	if err := os.Setenv("XDG_DATA_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// devtrackDataHome appends /devtrack, so place the state at that path.
	resolvedHome, err := devtrackDataHome()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeServerBootstrapState(resolvedHome, state); err != nil {
		t.Fatal(err)
	}
	printBootstrapCapabilities(&out)
	if got := out.String(); !strings.Contains(got, "Go-native") || !strings.Contains(got, "devtrack doctor --repair") {
		t.Fatalf("capability output did not preserve local readiness and recovery guidance:\n%s", got)
	}
}

func TestServerBootstrapStateAtomicRoundTrip(t *testing.T) {
	home := t.TempDir()
	state := &serverBootstrapState{Status: "queued", ProjectRoot: "/srv/devtrack"}
	if err := writeServerBootstrapState(home, state); err != nil {
		t.Fatal(err)
	}
	state.Status = "running"
	state.Step = "syncing"
	if err := writeServerBootstrapState(home, state); err != nil {
		t.Fatal(err)
	}
	got, err := readServerBootstrapState(home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "running" || got.Step != "syncing" {
		t.Fatalf("round-trip state = %+v", got)
	}
	if matches, err := filepath.Glob(filepath.Join(home, ".server-bootstrap-*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary state files remain: %v (err %v)", matches, err)
	}
}
