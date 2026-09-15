package hooks

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

// MaxCodexHistoryItemBytes bounds an internal Codex history row before it is
// decoded. The history adapter is a read-only SAGE-002 responsibility.
const MaxCodexHistoryItemBytes = 128 * 1024

type codexHistoryItem struct {
	Type           string `json:"type"`
	Status         string `json:"status"`
	ExitCode       *int   `json:"exitCode"`
	CWD            string `json:"cwd"`
	CommandActions []struct {
		Command string `json:"command"`
	} `json:"commandActions"`
}

// IsSupportedCodexHistorySource reports whether a Codex thread source is part
// of the compatibility-tested capture surface. "cli" includes Codex sessions
// launched by terminal integrations; no particular terminal is assumed.
func IsSupportedCodexHistorySource(source string) bool {
	return source == "vscode" || source == "cli"
}

// NormalizeCodexHistoryItem converts a stable commandExecution item into
// privacy-minimized Sage facts. In-progress, malformed, ambiguous, and unsafe
// items are ignored. Raw output and wrapper commands are never copied.
func NormalizeCodexHistoryItem(raw []byte, source, threadID, itemID string, receivedAt time.Time) []sage.Event {
	if len(raw) == 0 || len(raw) > MaxCodexHistoryItemBytes ||
		!IsSupportedCodexHistorySource(source) || threadID == "" || itemID == "" || receivedAt.IsZero() {
		return nil
	}

	var item codexHistoryItem
	if err := json.Unmarshal(raw, &item); err != nil || item.Type != "commandExecution" ||
		(item.Status != "completed" && item.Status != "failed") {
		return nil
	}

	exitCode := 0
	if item.Status == "failed" {
		exitCode = 1
	}
	if item.ExitCode != nil {
		exitCode = *item.ExitCode
	}
	success := item.Status == "completed"
	if (exitCode == 0) != success {
		return nil
	}

	seen := make(map[string]bool)
	events := make([]sage.Event, 0, len(item.CommandActions))
	for actionIndex, action := range item.CommandActions {
		if strings.TrimSpace(action.Command) == "" || seen[action.Command] {
			continue
		}
		seen[action.Command] = true
		command, signature, ok := safeCodexCommand(action.Command)
		if !ok {
			continue
		}
		event := sage.Event{
			SchemaVersion: sage.SchemaVersion,
			EventID:       opaqueID(threadID + "\x00" + itemID + "\x00" + strconv.Itoa(actionIndex)),
			Harness:       "codex-history",
			SessionID:     opaqueID(threadID),
			EventType:     "command",
			Tool:          "shell",
			OccurredAt:    receivedAt.UTC(),
			Command:       command,
			Signature:     signature,
			Success:       &success,
			ExitCode:      &exitCode,
		}
		if item.CWD != "" {
			event.ProjectID = opaqueID(item.CWD)
		}
		events = append(events, event)
	}
	return events
}
