package hooks

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/capture"
)

var devinTools = map[string]bool{
	"exec": true, "grep": true, "glob": true, "edit": true,
	"write": true, "apply_patch": true,
}

// NormalizeDevinEvents converts bounded PostToolUse payloads into validated,
// privacy-minimal Sage events.
func NormalizeDevinEvents(raw []byte, receivedAt time.Time) []sage.Event {
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
	toolName := strings.ToLower(strings.ReplaceAll(stringField(payload, "tool_name", "toolName", "tool"), "-", "_"))
	if !devinTools[toolName] {
		return nil
	}
	records := capture.RecordsFromPayload(payload, "devin")
	if len(records) == 0 {
		return nil
	}
	projectSource := stringField(payload, "cwd", "workingDirectory", "working_directory", "workspace_root", "workspaceRoot", "project_dir")
	sessionSource := stringField(payload, "session_id", "sessionId", "sessionID", "thread_id", "threadId")
	if sessionSource == "" {
		sessionSource = "devin-local\x00" + projectSource
	}
	events := make([]sage.Event, 0, len(records))
	for index, record := range records {
		identity := stringField(payload, "tool_use_id", "toolUseId", "tool_call_id", "toolCallId", "call_id", "callId")
		if identity == "" {
			identity = toolName + "\x00" + record.Signature + "\x00" + strconv.FormatBool(record.Success)
		}
		identity = sessionSource + "\x00" + identity
		if len(records) > 1 {
			identity += "\x00" + strconv.Itoa(index)
		}
		success := record.Success
		event := sage.Event{
			SchemaVersion: sage.SchemaVersion, EventID: opaqueID(identity), Harness: "devin",
			SessionID: opaqueID(sessionSource), EventType: "command", Tool: "agent-tool",
			OccurredAt: receivedAt.UTC(), Command: record.Command, Signature: record.Signature,
			Success: &success,
		}
		if record.Kind == "shell" {
			event.Tool = "shell"
		}
		if projectSource != "" {
			event.ProjectID = opaqueID(projectSource)
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
