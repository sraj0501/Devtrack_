// Package hooks contains pure harness-to-Sage normalization. It does not
// install hooks or write a spool; those are separate SAGE-002 responsibilities.
package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	sagecommand "github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/command"
)

// MaxCodexHookBytes caps the untrusted PostToolUse payload, which may include
// full model-facing tool output. Oversized calls are silently not captured.
const MaxCodexHookBytes = 128 * 1024

type codexPostToolUse struct {
	SessionID     string `json:"session_id"`
	CWD           string `json:"cwd"`
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	ToolUseID     string `json:"tool_use_id"`
	ToolInput     struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// NormalizeCodexPostToolUse accepts only a small, privacy-safe Bash subset.
// It never carries tool_response, raw command arguments, transcript paths, or
// cwd into the normalized event. A false result is a silent hook no-op.
func NormalizeCodexPostToolUse(raw []byte, receivedAt time.Time) (sage.Event, bool) {
	events := NormalizeCodexPostToolUseEvents(raw, receivedAt)
	if len(events) != 1 {
		return sage.Event{}, false
	}
	return events[0], true
}

// NormalizeCodexPostToolUseEvents returns one privacy-safe event per command
// segment. The plural form is used by the hook so pipelines do not collapse
// into one action; the singular compatibility helper above remains strict.
func NormalizeCodexPostToolUseEvents(raw []byte, receivedAt time.Time) []sage.Event {
	if len(raw) == 0 || len(raw) > MaxCodexHookBytes || receivedAt.IsZero() {
		return nil
	}
	var input codexPostToolUse
	if err := json.Unmarshal(raw, &input); err != nil ||
		input.HookEventName != "PostToolUse" || input.ToolName != "Bash" ||
		input.SessionID == "" || input.ToolUseID == "" {
		return nil
	}
	actions := safeCodexCommands(input.ToolInput.Command)
	if len(actions) == 0 {
		return nil
	}
	events := make([]sage.Event, 0, len(actions))
	for index, action := range actions {
		identity := input.SessionID + "\x00" + input.ToolUseID
		if len(actions) > 1 {
			identity += "\x00" + strconv.Itoa(index)
		}
		event := sage.Event{
			SchemaVersion: sage.SchemaVersion,
			EventID:       opaqueID(identity),
			Harness:       "codex",
			SessionID:     opaqueID(input.SessionID),
			EventType:     "command",
			Tool:          "shell",
			OccurredAt:    receivedAt.UTC(),
			Command:       action.Command,
			Signature:     action.Signature,
		}
		if input.CWD != "" {
			event.ProjectID = opaqueID(input.CWD)
		}
		events = append(events, event)
	}
	// PostToolUse includes failed shell calls, but its documented response is
	// tool-specific. Do not guess success or exit code before observed fixtures.
	return events
}

func opaqueID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

var unsafeCodexInput = regexp.MustCompile(`(?i)(?:password|passwd|api[_-]?key|secret|token|authorization|bearer|[a-z]:\\users\\|/home/|/users/)`)

func safeCodexCommand(raw string) (command, signature string, ok bool) {
	actions := safeCodexCommands(raw)
	if len(actions) != 1 {
		return "", "", false
	}
	return actions[0].Command, actions[0].Signature, true
}

func safeCodexCommands(raw string) []sagecommand.Action {
	if raw == "" || len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\x00`$><") || unsafeCodexInput.MatchString(raw) {
		return nil
	}
	return sagecommand.Parse(raw)
}
