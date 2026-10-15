package hooks

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/capture"
)

// NormalizeCopilotEvents converts Copilot CLI postToolUse input into bounded,
// privacy-safe Sage facts. Raw results and argument values are never copied.
func NormalizeCopilotEvents(raw []byte, receivedAt time.Time) []sage.Event {
	if len(raw) == 0 || len(raw) > MaxHarnessHookBytes || receivedAt.IsZero() {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	if eventName := stringField(payload, "hookEventName", "hook_event_name", "eventName"); eventName != "" && !strings.EqualFold(eventName, "PostToolUse") {
		return nil
	}
	sessionID := stringField(payload, "sessionId", "sessionID", "session_id")
	toolName := stringField(payload, "toolName", "tool_name", "tool")
	if sessionID == "" || !copilotToolAllowed(toolName) {
		return nil
	}
	records := capture.RecordsFromPayload(payload, "copilot")
	events := make([]sage.Event, 0, len(records))
	for index, record := range records {
		identity := stringField(payload, "toolCallId", "toolUseId", "tool_use_id", "callId", "timestamp")
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
			Harness:       "copilot",
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

func stringField(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && value != "" {
			return value
		}
		if value, ok := payload[key].(float64); ok {
			return fmt.Sprint(value)
		}
	}
	return ""
}

func copilotToolAllowed(tool string) bool {
	tool = strings.ToLower(strings.ReplaceAll(tool, "-", "_"))
	allowed := map[string]bool{
		"exec": true, "bash": true, "shell": true, "run": true, "runcommand": true,
		"run_command": true, "terminal": true, "run_terminal_cmd": true,
		"execute_command": true, "executecommand": true, "launch_process": true,
		"shell_command": true, "bashtool": true, "command": true, "process": true,
		"grep": true, "search": true, "ripgrep": true, "file_search": true,
		"search_files": true, "searchtext": true, "codebase_search": true,
		"glob": true, "find_files": true, "file_glob": true, "list_files": true,
		"listfiles": true, "edit": true, "multiedit": true, "multi_edit": true,
		"str_replace": true, "str_replace_editor": true, "write": true,
		"create_file": true, "createfile": true, "apply_patch": true,
		"applypatch": true, "patch": true, "notebook_edit": true, "notebookedit": true,
	}
	return allowed[tool]
}
