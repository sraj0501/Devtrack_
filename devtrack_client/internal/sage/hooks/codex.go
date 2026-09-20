// Package hooks contains pure harness-to-Sage normalization. It does not
// install hooks or write a spool; those are separate SAGE-002 responsibilities.
package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
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
	if len(raw) == 0 || len(raw) > MaxCodexHookBytes || receivedAt.IsZero() {
		return sage.Event{}, false
	}
	var input codexPostToolUse
	if err := json.Unmarshal(raw, &input); err != nil ||
		input.HookEventName != "PostToolUse" || input.ToolName != "Bash" ||
		input.SessionID == "" || input.ToolUseID == "" {
		return sage.Event{}, false
	}
	command, signature, ok := safeCodexCommand(input.ToolInput.Command)
	if !ok {
		return sage.Event{}, false
	}
	event := sage.Event{
		SchemaVersion: sage.SchemaVersion,
		EventID:       opaqueID(input.SessionID + "\x00" + input.ToolUseID),
		Harness:       "codex",
		SessionID:     opaqueID(input.SessionID),
		EventType:     "command",
		Tool:          "shell",
		OccurredAt:    receivedAt.UTC(),
		Command:       command,
		Signature:     signature,
	}
	if input.CWD != "" {
		event.ProjectID = opaqueID(input.CWD)
	}
	// PostToolUse includes failed shell calls, but its documented response is
	// tool-specific. Do not guess success or exit code before observed fixtures.
	return event, true
}

func opaqueID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

var safeActions = map[string]map[string]bool{
	"git": {"status": true, "log": true, "diff": true, "add": true, "commit": true,
		"push": true, "pull": true, "fetch": true, "branch": true, "switch": true,
		"checkout": true, "merge": true, "rebase": true, "stash": true},
	"go":  {"test": true, "build": true, "vet": true, "fmt": true},
	"npm": {"test": true, "run": true, "install": true},
	"uv":  {"sync": true, "run": true, "pip": true},
}

var safeFlags = map[string]bool{
	"--short": true, "--porcelain": true, "--stat": true,
	"--oneline": true, "--dry-run": true, "-v": true, "-race": true,
}

var unsafeCodexInput = regexp.MustCompile(`(?i)(?:password|passwd|api[_-]?key|secret|token|authorization|bearer|[a-z]:\\users\\|/home/|/users/)`)

func safeCodexCommand(raw string) (command, signature string, ok bool) {
	if raw == "" || len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\x00;&|`$><\"'\\") || unsafeCodexInput.MatchString(raw) {
		return "", "", false
	}
	parts := strings.Fields(raw)
	if len(parts) < 2 || !safeActions[parts[0]][parts[1]] {
		return "", "", false
	}
	signature = parts[0] + " " + parts[1]
	command = signature
	for _, part := range parts[2:] {
		if safeFlags[part] {
			command += " " + part
		}
	}
	return command, signature, true
}
