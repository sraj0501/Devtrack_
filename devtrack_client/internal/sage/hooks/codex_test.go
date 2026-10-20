package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func codexFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/codex-post-tool-use-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestNormalizeCodexPostToolUseMinimizesPrivateInput(t *testing.T) {
	raw := codexFixture(t)
	when := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	event, ok := NormalizeCodexPostToolUse(raw, when)
	if !ok || event.Command != "git status --short" || event.Signature != "git status --short" {
		t.Fatalf("unexpected normalized event: %+v, %t", event, ok)
	}
	if event.Success != nil || event.ExitCode != nil {
		t.Fatal("undocumented response shape must not imply a result")
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"example-session", "/workspace/", "PRIVATE_OUTPUT_CANARY", "/private/", "example-tool-001"} {
		if bytes.Contains(encoded, []byte(private)) {
			t.Fatalf("normalized event leaked %q", private)
		}
	}
	if _, err := sage.DecodeEvent(encoded); err != nil {
		t.Fatalf("normalized event violates Sage v1: %v", err)
	}
	repeated, ok := NormalizeCodexPostToolUse(raw, when.Add(time.Minute))
	if !ok || event.DeliveryKey() != repeated.DeliveryKey() {
		t.Fatal("repeated delivery changed key")
	}
	distinctRaw := bytes.Replace(raw, []byte(`example-tool-001`), []byte(`example-tool-002`), 1)
	distinct, ok := NormalizeCodexPostToolUse(distinctRaw, when)
	if !ok || event.DeliveryKey() == distinct.DeliveryKey() {
		t.Fatal("distinct tool call reused delivery key")
	}
}

func TestNormalizeCodexPostToolUseFailsClosed(t *testing.T) {
	fixture := codexFixture(t)
	when := time.Now()
	cases := map[string][]byte{
		"malformed":       []byte(`{`),
		"oversized":       bytes.Repeat([]byte("x"), MaxCodexHookBytes+1),
		"wrong event":     bytes.Replace(fixture, []byte(`PostToolUse`), []byte(`PreToolUse`), 1),
		"wrong tool":      bytes.Replace(fixture, []byte(`"Bash"`), []byte(`"apply_patch"`), 1),
		"missing tool id": bytes.Replace(fixture, []byte(`example-tool-001`), []byte(``), 1),
		"credential":      bytes.Replace(fixture, []byte(`git status --short`), []byte(`git status --token=PRIVATE`), 1),
		"private path":    bytes.Replace(fixture, []byte(`git status --short`), []byte(`git status /home/alice`), 1),
		"shell expansion": bytes.Replace(fixture, []byte(`git status --short`), []byte(`git status; echo $private`), 1),
		"redirection":     bytes.Replace(fixture, []byte(`git status --short`), []byte(`git status > private`), 1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if event, ok := NormalizeCodexPostToolUse(raw, when); ok {
				t.Fatalf("unsafe input was captured: %+v", event)
			}
		})
	}
}

func TestSafeCodexCommandDropsValues(t *testing.T) {
	command, signature, ok := safeCodexCommand("go test ./... -race")
	if !ok || command != "go test -race" || signature != "go test -race" {
		t.Fatalf("unexpected safe command: %q, %q, %t", command, signature, ok)
	}
	if strings.Contains(command, "./...") {
		t.Fatal("path escaped into display command")
	}
}

func TestReferenceSameActionDifferentValuesCollapses(t *testing.T) {
	_, left, okLeft := safeCodexCommand("go test ./first/package -race")
	_, right, okRight := safeCodexCommand("go test ./second/package -race")
	if !okLeft || !okRight || left != "go test -race" || right != left {
		t.Fatalf("left=%q right=%q ok=%v/%v", left, right, okLeft, okRight)
	}
}

func TestNormalizeCodexPostToolUseEmitsEveryCompoundAction(t *testing.T) {
	raw := bytes.Replace(codexFixture(t), []byte(`git status --short`), []byte(`git fetch upstream && git merge upstream/dev`), 1)
	events := NormalizeCodexPostToolUseEvents(raw, time.Now())
	if len(events) != 2 || events[0].Signature != "git fetch" || events[1].Signature != "git merge" || events[0].EventID == events[1].EventID {
		t.Fatalf("events=%+v", events)
	}
}

func TestReferenceNamesDoNotLeakIntoSignatures(t *testing.T) {
	command, signature, ok := safeCodexCommand("git checkout private-customer-branch")
	if !ok || signature != "git checkout" || strings.Contains(command, "private-customer") || strings.Contains(signature, "private-customer") {
		t.Fatalf("command=%q signature=%q ok=%v", command, signature, ok)
	}
}

func TestReferenceRealSubcommandsStillSplit(t *testing.T) {
	_, status, okStatus := safeCodexCommand("git status --short")
	_, logSignature, okLog := safeCodexCommand("git log --oneline")
	if !okStatus || !okLog || status == logSignature {
		t.Fatalf("status=%q log=%q", status, logSignature)
	}
}

func TestReferenceDistinctActionsStayDistinct(t *testing.T) {
	_, gitSignature, okGit := safeCodexCommand("git status")
	_, goSignature, okGo := safeCodexCommand("go test ./...")
	if !okGit || !okGo || gitSignature == goSignature {
		t.Fatalf("git=%q go=%q", gitSignature, goSignature)
	}
}

func TestReferenceTrivialCommandsAreIgnored(t *testing.T) {
	for _, command := range []string{"pwd", "ls", "echo hello", "cd project"} {
		if event, signature, ok := safeCodexCommand(command); ok {
			t.Fatalf("trivial command accepted: %q %q %q", command, event, signature)
		}
	}
}
