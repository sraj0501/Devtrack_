package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func codexHistoryFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/codex-history-command-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestNormalizeCodexHistoryItemSupportsIDEAndCLI(t *testing.T) {
	raw := codexHistoryFixture(t)
	when := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	for _, source := range []string{"vscode", "cli"} {
		t.Run(source, func(t *testing.T) {
			events := NormalizeCodexHistoryItem(raw, source, "example-thread", "example-item", when)
			if len(events) != 1 {
				t.Fatalf("got %d events, want 1", len(events))
			}
			event := events[0]
			if event.Command != "git status --short" || event.Signature != "git status" ||
				event.Success == nil || !*event.Success || event.ExitCode == nil || *event.ExitCode != 0 {
				t.Fatalf("unexpected normalized event: %+v", event)
			}
			encoded, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			for _, private := range []string{"example-thread", "example-item", "PRIVATE_OUTPUT_CANARY", "/private/repo"} {
				if bytes.Contains(encoded, []byte(private)) {
					t.Fatalf("normalized event leaked %q", private)
				}
			}
			if _, err := sage.DecodeEvent(encoded); err != nil {
				t.Fatalf("normalized event violates Sage v1: %v", err)
			}
		})
	}
}

func TestNormalizeCodexHistoryItemPreservesFailureAndDeduplicates(t *testing.T) {
	raw := bytes.Replace(codexHistoryFixture(t), []byte(`"status": "completed"`), []byte(`"status": "failed"`), 1)
	raw = bytes.Replace(raw, []byte(`"exitCode": 0`), []byte(`"exitCode": 7`), 1)
	raw = bytes.Replace(raw, []byte(`"commandActions": [`), []byte(`"commandActions": [{"command":"git status --short"},`), 1)
	events := NormalizeCodexHistoryItem(raw, "cli", "thread", "item", time.Now())
	if len(events) != 1 || events[0].Success == nil || *events[0].Success ||
		events[0].ExitCode == nil || *events[0].ExitCode != 7 {
		t.Fatalf("unexpected failure event: %+v", events)
	}
}

func TestNormalizeCodexHistoryItemFailsClosed(t *testing.T) {
	fixture := codexHistoryFixture(t)
	when := time.Now()
	cases := map[string]struct {
		raw, source string
	}{
		"unsupported source": {string(fixture), "warp"},
		"in progress":        {string(bytes.Replace(fixture, []byte(`completed`), []byte(`inProgress`), 1)), "cli"},
		"wrong type":         {string(bytes.Replace(fixture, []byte(`commandExecution`), []byte(`userMessage`), 1)), "cli"},
		"conflicting result": {string(bytes.Replace(fixture, []byte(`"exitCode": 0`), []byte(`"exitCode": 2`), 1)), "cli"},
		"unsafe command":     {string(bytes.Replace(fixture, []byte(`git status --short`), []byte(`git status --token=PRIVATE`), 1)), "cli"},
		"malformed":          {`{`, "cli"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if events := NormalizeCodexHistoryItem([]byte(test.raw), test.source, "thread", "item", when); len(events) != 0 {
				t.Fatalf("unsafe history item was captured: %+v", events)
			}
		})
	}
}
