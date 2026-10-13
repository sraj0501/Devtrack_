// Package capture normalizes harness tool payloads into privacy-safe Sage
// records. It performs no filesystem, database, model, or network work.
package capture

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	sagecommand "github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/command"
)

var toolNameKeys = []string{"tool_name", "toolName", "tool", "name"}
var toolInputKeys = []string{"tool_input", "toolArgs", "tool_args", "toolInput", "arguments", "args", "input", "parameters", "params"}
var commandKeys = []string{"command", "cmd", "commandLine", "command_line", "script", "shell_command", "bash", "input"}

var shellTools = set("exec", "bash", "shell", "run", "runcommand", "run_command", "terminal", "run_terminal_cmd", "execute_command", "executecommand", "launch_process", "shell_command", "bashtool", "command", "process")
var grepTools = set("grep", "search", "ripgrep", "file_search", "search_files", "searchtext", "codebase_search")
var globTools = set("glob", "find_files", "file_glob", "list_files", "listfiles")
var editTools = map[string]string{
	"edit": "sed -i", "multiedit": "sed -i", "multi_edit": "sed -i",
	"str_replace": "sed -i", "str_replace_editor": "sed -i",
	"write": "cat", "create_file": "cat", "createfile": "cat",
	"apply_patch": "git apply", "applypatch": "git apply", "patch": "git apply",
	"notebook_edit": "jupyter nbconvert", "notebookedit": "jupyter nbconvert",
}

func set(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

// Record is the reusable action fact produced from one tool payload. Command
// and Signature omit paths, argument values, free text, and tool output.
type Record struct {
	Signature   string
	Binary      string
	Subcommands []string
	Command     string
	Harness     string
	Kind        string
	Tool        string
	Success     bool
}

// RecordsFromPayload duck-types the stable payload shapes emitted by the
// supported harness families. Unknown or incomplete inputs are ignored.
func RecordsFromPayload(payload map[string]any, harness string) []Record {
	rawTool, _ := first(payload, toolNameKeys...).(string)
	tool := normalizeTool(rawTool)
	args := asMap(first(payload, toolInputKeys...))
	if len(args) == 0 {
		args = payload
	}
	success := succeeded(payload)

	if command := commandValue(first(args, commandKeys...)); command != "" && (shellTools[tool] || editTools[tool] == "") {
		actions := sagecommand.Parse(command)
		out := make([]Record, 0, len(actions))
		for _, action := range actions {
			out = append(out, Record{Signature: action.Signature, Binary: action.Binary,
				Subcommands: action.Subcommands, Command: action.Command, Harness: harness,
				Kind: "shell", Tool: rawTool, Success: success})
		}
		if len(out) > 0 {
			return out
		}
	}

	if grepTools[tool] || globTools[tool] {
		command := ripgrepCommand(tool, args)
		actions := sagecommand.Parse(command)
		if len(actions) == 0 {
			return nil
		}
		action := actions[0]
		return []Record{{Signature: action.Signature, Binary: action.Binary,
			Subcommands: action.Subcommands, Command: action.Command, Harness: harness,
			Kind: "agent-tool", Tool: rawTool, Success: success}}
	}

	if equivalent := editTools[tool]; equivalent != "" {
		actions := sagecommand.Parse(equivalent)
		if len(actions) == 0 {
			return nil
		}
		action := actions[0]
		return []Record{{Signature: "agent-tool " + tool, Binary: tool,
			Command: action.Command, Harness: harness, Kind: "agent-tool", Tool: rawTool,
			Success: success}}
	}
	return nil
}

func first(value map[string]any, keys ...string) any {
	for _, key := range keys {
		if candidate, ok := value[key]; ok && candidate != nil && candidate != "" {
			return candidate
		}
	}
	return nil
}

func asMap(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return nil
	}
	var result map[string]any
	if json.Unmarshal([]byte(text), &result) != nil {
		return nil
	}
	return result
}

func commandValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, part := range typed {
			parts = append(parts, fmt.Sprint(part))
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	default:
		return ""
	}
}

func normalizeTool(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "-", "_"))
	if parts := strings.Split(value, "__"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return value
}

func succeeded(payload map[string]any) bool {
	for _, key := range []string{"tool_response", "toolResponse", "tool_result", "toolResult", "result", "output", "response"} {
		node, found := payload[key]
		if !found {
			continue
		}
		if object, ok := node.(map[string]any); ok {
			if value, ok := object["success"].(bool); ok {
				return value
			}
			for _, resultKey := range []string{"result_type", "resultType", "status", "type"} {
				if value, ok := object[resultKey].(string); ok {
					switch strings.ToLower(value) {
					case "success", "ok", "succeeded":
						return true
					default:
						return false
					}
				}
			}
			if value, ok := object["is_error"].(bool); ok {
				return !value
			}
			if value, ok := object["error"]; ok && value != nil && value != "" {
				return false
			}
		} else if _, ok := node.(string); ok && (key == "result" || key == "output") {
			return true
		}
	}
	for _, key := range []string{"exit_code", "exitCode", "status_code"} {
		if value, ok := number(payload[key]); ok {
			return value == 0
		}
	}
	if payload["hook_event_name"] == "PostToolUseFailure" || payload["event"] == "errorOccurred" {
		return false
	}
	return true
}

func number(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), typed == float64(int(typed))
	default:
		return 0, false
	}
}

func ripgrepCommand(tool string, args map[string]any) string {
	pattern := text(first(args, "pattern", "query", "regex", "search", "text"))
	if globTools[tool] {
		if pattern == "" {
			pattern = text(first(args, "glob", "glob_pattern", "globPattern", "path"))
		}
		if pattern == "" {
			return ""
		}
		return "rg --files --glob value"
	}
	if pattern == "" {
		return ""
	}
	flags := []string{"--regexp"}
	if truthy(first(args, "case_insensitive", "-i", "ignore_case")) {
		flags = append(flags, "--ignore-case")
	}
	if first(args, "context_lines", "-C", "context") != nil {
		flags = append(flags, "--context")
	}
	if truthy(first(args, "-n", "line_numbers")) {
		flags = append(flags, "--line-number")
	}
	switch text(first(args, "output_mode", "outputMode")) {
	case "files_with_matches":
		flags = append(flags, "--files-with-matches")
	case "count":
		flags = append(flags, "--count")
	}
	if first(args, "glob_pattern", "glob", "globPattern", "include", "type") != nil {
		flags = append(flags, "--glob")
	}
	sort.Strings(flags)
	return "rg " + strings.Join(flags, " ")
}

func text(value any) string {
	result, _ := value.(string)
	return result
}

func truthy(value any) bool {
	result, _ := value.(bool)
	return result
}
