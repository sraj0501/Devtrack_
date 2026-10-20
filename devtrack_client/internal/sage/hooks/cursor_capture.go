package hooks

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/capture"
)

// NormalizeCursorEvents converts Cursor afterShellExecution and postToolUse
// payloads into bounded, privacy-safe Sage events.
func NormalizeCursorEvents(raw []byte, receivedAt time.Time) []sage.Event {
	if len(raw) == 0 || len(raw) > MaxHarnessHookBytes || receivedAt.IsZero() {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	eventName := stringField(payload, "hook_event_name", "hookEventName", "eventName", "event")
	if eventName != "" && !strings.EqualFold(eventName, "afterShellExecution") && !strings.EqualFold(eventName, "postToolUse") {
		return nil
	}
	toolName := stringField(payload, "tool_name", "toolName", "tool")
	if toolName == "" && stringField(payload, "command", "cmd", "commandLine", "command_line") != "" {
		toolName = "shell"
	}
	if !harnessToolAllowed(toolName) {
		return nil
	}
	normalizedTool := strings.ToLower(strings.ReplaceAll(toolName, "-", "_"))
	records := capture.RecordsFromPayload(payload, "cursor")
	if len(records) == 0 {
		return nil
	}
	sessionSource := stringField(payload, "session_id", "sessionId", "sessionID", "conversation_id", "conversationId", "thread_id", "threadId")
	projectSource := stringField(payload, "cwd", "workingDirectory", "working_directory", "workspace_root", "workspaceRoot", "project_dir")
	if sessionSource == "" {
		sessionSource = "cursor-local\x00" + projectSource
	}
	events := make([]sage.Event, 0, len(records))
	for index, record := range records {
		identity := stringField(payload, "tool_use_id", "toolUseId", "tool_call_id", "toolCallId", "call_id", "callId", "generation_id", "generationId")
		if identity == "" {
			identity = normalizedTool + "\x00" + record.Signature + "\x00" + strconv.FormatBool(record.Success)
		}
		identity = sessionSource + "\x00" + identity
		if len(records) > 1 {
			identity += "\x00" + strconv.Itoa(index)
		}
		success := record.Success
		event := sage.Event{
			SchemaVersion: sage.SchemaVersion,
			EventID:       opaqueID(identity),
			Harness:       "cursor",
			SessionID:     opaqueID(sessionSource),
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
