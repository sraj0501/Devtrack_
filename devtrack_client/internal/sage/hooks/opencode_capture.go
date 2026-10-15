package hooks

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/capture"
)

// NormalizeOpenCodeEvents converts plugin tool payloads into bounded,
// privacy-safe Sage events. Before/after duplicates share the same fallback
// identity and collapse in the immutable spool/import path.
func NormalizeOpenCodeEvents(raw []byte, receivedAt time.Time) []sage.Event {
	if len(raw) == 0 || len(raw) > MaxHarnessHookBytes || receivedAt.IsZero() {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	if eventName := stringField(payload, "hook_event_name", "hookEventName", "eventName"); eventName != "" && !strings.EqualFold(eventName, "PostToolUse") {
		return nil
	}
	sessionID := stringField(payload, "session_id", "sessionID", "sessionId")
	toolName := stringField(payload, "tool_name", "toolName", "tool")
	if sessionID == "" || !harnessToolAllowed(toolName) {
		return nil
	}
	records := capture.RecordsFromPayload(payload, "opencode")
	events := make([]sage.Event, 0, len(records))
	for index, record := range records {
		identity := stringField(payload, "call_id", "callId", "tool_call_id", "toolCallId")
		if identity == "" {
			identity = toolName + "\x00" + record.Signature + "\x00" + strconv.FormatBool(record.Success)
		}
		identity = sessionID + "\x00" + identity
		if len(records) > 1 {
			identity += "\x00" + strconv.Itoa(index)
		}
		success := record.Success
		event := sage.Event{
			SchemaVersion: sage.SchemaVersion,
			EventID:       opaqueID(identity),
			Harness:       "opencode",
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
		if cwd := stringField(payload, "cwd", "workingDirectory", "working_directory"); cwd != "" {
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
