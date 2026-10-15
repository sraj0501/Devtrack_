package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/hooks"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

// TestSageCaptureToKnowledgeAcceptanceTwice is the SAGE-003 vertical gate.
// The ordinary client CI matrix runs both clean passes natively on Windows and
// Linux. A local fake Ollama endpoint makes outage recovery deterministic and
// keeps the acceptance test offline.
func TestSageCaptureToKnowledgeAcceptanceTwice(t *testing.T) {
	for pass := 1; pass <= 2; pass++ {
		t.Run(fmt.Sprintf("clean-pass-%d", pass), testSageCaptureToKnowledgeJourney)
	}
}

func testSageCaptureToKnowledgeJourney(t *testing.T) {
	t.Helper()
	dataHome := t.TempDir()
	root := filepath.Join(dataHome, "devtrack", "sage")
	codexHome := filepath.Join(t.TempDir(), "codex")
	repo := sageAcceptanceRepository(t)
	knowledgeRoot := filepath.Join(repo, "knowledge")
	if err := os.MkdirAll(codexHome, 0700); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(codexHome, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(`{"custom":"keep","hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"user-owned-hook"}]}]}}`), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("DEVTRACK_SAGE_KNOWLEDGE_DIR", knowledgeRoot)
	t.Setenv("DEVTRACK_SAGE_IDLE_POLL_MS", "50")
	t.Setenv("DEVTRACK_SAGE_RETRY_DELAY_SECS", "1")
	t.Setenv("DEVTRACK_SAGE_AUTO_COMMIT", "true")
	t.Setenv("GIT_NO_DEVTRACK", "1")

	installer := hooks.Installer{Root: root, CodexHome: codexHome, Executable: "devtrack-test", GOOS: runtime.GOOS}
	installed, err := installer.Install()
	if err != nil || !installed.Installed {
		t.Fatalf("install: result=%+v err=%v", installed, err)
	}
	assertContainsFile(t, hooksPath, "user-owned-hook")

	payload := []byte(`{"session_id":"acceptance-thread","cwd":"C:/private/SAGE_PATH_CANARY","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"acceptance-call","tool_input":{"command":"git status --short"},"tool_response":"SAGE_PRIVATE_CANARY"}`)
	events := hooks.NormalizeCodexPostToolUseEvents(payload, time.Now().UTC())
	if len(events) != 1 || events[0].Command != "git status --short" {
		t.Fatalf("normalized events=%+v", events)
	}
	normalized, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	assertNoAcceptanceCanary(t, normalized)
	result, err := sage.WriteEvents(root, events)
	if err != nil || result.New != 1 {
		t.Fatalf("silent capture: result=%+v err=%v", result, err)
	}

	var distillCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "You organise a personal command reference") {
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": `{"file":"shell.md","title":"Shell"}`}, "done": true})
			return
		}
		switch distillCalls.Add(1) {
		case 1:
			http.Error(w, "SAGE_PRIVATE_CANARY outage", http.StatusServiceUnavailable)
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": "malformed model output"}, "done": true})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": `{"worth_documenting":true,"title":"Inspect repository state","section":"Git","commands":["git status --short"],"what":"Shows repository changes.","why":"Review changes before committing.","example":"git status --short","notes":""}`}, "done": true})
		}
	}))
	defer server.Close()
	t.Setenv("OLLAMA_HOST", server.URL)

	database, err := db.NewDatabaseAtPath(filepath.Join(dataHome, "devtrack", "acceptance.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	im := &IntegratedMonitor{database: database}
	im.startSageCapture(context.Background())
	waitForSageAcceptance(t, database)
	waitForSageAcceptanceCommit(t, repo)
	im.sageCancel()
	<-im.sageDone

	var eventCount, jobCount int
	if err := database.DB().QueryRow(`SELECT count(*) FROM sage_events`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := database.DB().QueryRow(`SELECT count(*) FROM sage_jobs`).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 || jobCount != 1 || distillCalls.Load() != 3 {
		t.Fatalf("eventCount=%d jobCount=%d distillCalls=%d", eventCount, jobCount, distillCalls.Load())
	}

	writer := knowledge.Writer{Root: knowledgeRoot}
	entries, err := database.SearchSageEntries(context.Background(), writer, "repository changes", "shell", 20)
	if err != nil || len(entries) != 1 {
		t.Fatalf("search before correction: entries=%+v err=%v", entries, err)
	}
	if err := database.RememberSageRoute(context.Background(), writer, "git", "version-control"); err != nil {
		t.Fatal(err)
	}
	var remembered map[string]string
	if err := json.Unmarshal(readAcceptanceFile(t, filepath.Join(knowledgeRoot, ".routes.json")), &remembered); err != nil || remembered["git"] != "version-control.md" {
		t.Fatalf("remembered routes=%+v err=%v", remembered, err)
	}
	moved, err := database.MergeSageTopics(context.Background(), writer, "shell", "version-control")
	if err != nil || moved != 1 {
		t.Fatalf("merge: moved=%d err=%v", moved, err)
	}
	entries, err = database.SearchSageEntries(context.Background(), writer, "git status", "version-control", 20)
	if err != nil || len(entries) != 1 {
		t.Fatalf("search after merge: entries=%+v err=%v", entries, err)
	}
	route, found, err := writer.Route("git")
	if err != nil || !found || route.Filename != "version-control.md" {
		t.Fatalf("effective route=%+v found=%t err=%v", route, found, err)
	}

	firstTopic := readAcceptanceFile(t, filepath.Join(knowledgeRoot, "version-control.md"))
	firstIndex := readAcceptanceFile(t, filepath.Join(knowledgeRoot, "README.md"))
	restarted := &IntegratedMonitor{database: database}
	restarted.startSageCapture(context.Background())
	time.Sleep(200 * time.Millisecond)
	restarted.sageCancel()
	<-restarted.sageDone
	if !bytes.Equal(firstTopic, readAcceptanceFile(t, filepath.Join(knowledgeRoot, "version-control.md"))) ||
		!bytes.Equal(firstIndex, readAcceptanceFile(t, filepath.Join(knowledgeRoot, "README.md"))) {
		t.Fatal("restart changed deterministic knowledge bytes")
	}
	if distillCalls.Load() != 3 {
		t.Fatalf("restart repeated model work: calls=%d", distillCalls.Load())
	}

	assertSageAcceptanceGitState(t, repo)
	assertTreeHasNoAcceptanceCanary(t, root)
	assertTreeHasNoAcceptanceCanary(t, knowledgeRoot)
	uninstalled, err := installer.Uninstall()
	if err != nil || uninstalled.Installed {
		t.Fatalf("uninstall: result=%+v err=%v", uninstalled, err)
	}
	assertContainsFile(t, hooksPath, "user-owned-hook")
	if status, err := installer.Status(); err != nil || status.Installed {
		t.Fatalf("post-uninstall status=%+v err=%v", status, err)
	}
}

func waitForSageAcceptance(t *testing.T, database *db.Database) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var jobState, publicationState string
		var attempts int
		jobErr := database.DB().QueryRow(`SELECT state,attempts FROM sage_jobs`).Scan(&jobState, &attempts)
		publicationErr := database.DB().QueryRow(`SELECT state FROM sage_publications`).Scan(&publicationState)
		if jobErr == nil && publicationErr == nil && jobState == "distilled" && publicationState == "documented" {
			if attempts != 3 {
				t.Fatalf("model attempts=%d, want 3", attempts)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Sage acceptance journey did not publish before deadline")
}

func waitForSageAcceptanceCommit(t *testing.T, repo string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		committed := filepath.ToSlash(sageAcceptanceGit(t, repo, "show", "--name-only", "--format=", "HEAD"))
		staged := filepath.ToSlash(sageAcceptanceGit(t, repo, "diff", "--cached", "--name-only"))
		if strings.Contains(committed, "knowledge/") && staged == "staged.txt" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Sage knowledge commit did not finish before deadline")
}

func sageAcceptanceRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	sageAcceptanceGit(t, root, "init", "-q")
	sageAcceptanceGit(t, root, "config", "user.name", "Sage Acceptance")
	sageAcceptanceGit(t, root, "config", "user.email", "sage@example.invalid")
	for name, value := range map[string]string{"seed.txt": "seed\n", "staged.txt": "baseline\n", "unstaged.txt": "baseline\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sageAcceptanceGit(t, root, "add", "seed.txt", "staged.txt", "unstaged.txt")
	sageAcceptanceGit(t, root, "commit", "-q", "-m", "seed")
	if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("user staged work\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sageAcceptanceGit(t, root, "add", "staged.txt")
	if err := os.WriteFile(filepath.Join(root, "unstaged.txt"), []byte("user unstaged work\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func sageAcceptanceGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_NO_DEVTRACK=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func assertSageAcceptanceGitState(t *testing.T, repo string) {
	t.Helper()
	if got := filepath.ToSlash(sageAcceptanceGit(t, repo, "diff", "--cached", "--name-only")); got != "staged.txt" {
		t.Fatalf("staged user state changed: %q", got)
	}
	if got := filepath.ToSlash(sageAcceptanceGit(t, repo, "diff", "--name-only")); !strings.Contains(got, "unstaged.txt") {
		t.Fatalf("unstaged user state changed: %q", got)
	}
	committed := filepath.ToSlash(sageAcceptanceGit(t, repo, "show", "--name-only", "--format=", "HEAD"))
	if strings.Contains(committed, "staged.txt") || strings.Contains(committed, "unstaged.txt") || !strings.Contains(committed, "knowledge/") {
		t.Fatalf("unsafe Sage commit paths: %q", committed)
	}
}

func assertTreeHasNoAcceptanceCanary(t *testing.T, root string) {
	t.Helper()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		assertNoAcceptanceCanary(t, raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertNoAcceptanceCanary(t *testing.T, raw []byte) {
	t.Helper()
	for _, forbidden := range [][]byte{[]byte("SAGE_PRIVATE_CANARY"), []byte("SAGE_PATH_CANARY"), []byte("C:/private")} {
		if bytes.Contains(raw, forbidden) {
			t.Fatalf("privacy canary leaked: %q", forbidden)
		}
	}
}

func assertContainsFile(t *testing.T, path, value string) {
	t.Helper()
	if raw := readAcceptanceFile(t, path); !bytes.Contains(raw, []byte(value)) {
		t.Fatalf("%s does not contain preserved value %q", path, value)
	}
}

func readAcceptanceFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
