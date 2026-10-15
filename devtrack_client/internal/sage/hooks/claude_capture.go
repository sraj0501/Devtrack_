package hooks

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/capture"
)

// MaxHarnessHookBytes caps untrusted harness payloads before JSON decoding.
const MaxHarnessHookBytes = MaxCodexHookBytes

// NormalizeClaudeEvents converts supported Claude tool events into small,
// privacy-safe Sage facts. Tool output and argument values are never copied.
func NormalizeClaudeEvents(raw []byte, receivedAt time.Time) []sage.Event {
	if len(raw) == 0 || len(raw) > MaxHarnessHookBytes || receivedAt.IsZero() {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	eventName, _ := payload["hook_event_name"].(string)
	if eventName != "PostToolUse" && eventName != "PostToolUseFailure" {
		return nil
	}
	toolName, _ := payload["tool_name"].(string)
	allowed := map[string]bool{
		"Bash": true, "Grep": true, "Glob": true, "Edit": true,
		"MultiEdit": true, "Write": true, "NotebookEdit": true,
	}
	if !allowed[toolName] || (eventName == "PostToolUseFailure" && toolName != "Bash") {
		return nil
	}
	sessionID, _ := payload["session_id"].(string)
	toolUseID, _ := payload["tool_use_id"].(string)
	if sessionID == "" || toolUseID == "" {
		return nil
	}
	records := capture.RecordsFromPayload(payload, "claude")
	events := make([]sage.Event, 0, len(records))
	for index, record := range records {
		identity := sessionID + "\x00" + toolUseID
		if len(records) > 1 {
			identity += "\x00" + strconv.Itoa(index)
		}
		success := record.Success
		event := sage.Event{
			SchemaVersion: sage.SchemaVersion,
			EventID:       opaqueID(identity),
			Harness:       "claude",
			SessionID:     opaqueID(sessionID),
			EventType:     "command",
			Tool:          "agent-tool",
			OccurredAt:    receivedAt.UTC(),
			Command:       record.Command,
			Signature:     record.Signature,
			Success:       &success,
		}
		if record.Kind == "shell" {
			event.Tool = "shell"
		}
		if cwd, _ := payload["cwd"].(string); cwd != "" {
			event.ProjectID = opaqueID(cwd)
		}
		encoded, err := json.Marshal(event)
		if err == nil {
			if _, err := sage.DecodeEvent(encoded); err == nil {
				events = append(events, event)
			}
		}
	}
	return events
}
