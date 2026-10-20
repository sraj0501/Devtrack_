package capture

import (
	"encoding/json"
	"testing"
)

func TestReferenceEveryShapeProducesAShellRecord(t *testing.T) {
	cases := map[string]map[string]any{
		"devin":         {"tool_name": "exec", "tool_input": map[string]any{"command": "terraform apply"}, "tool_response": map[string]any{"success": true}},
		"claude":        {"hook_event_name": "PostToolUse", "tool_name": "Bash", "cwd": "/repo", "tool_input": map[string]any{"command": "helm upgrade api ./chart"}, "tool_response": map[string]any{"stdout": "ok"}},
		"codex":         {"tool_name": "Bash", "tool_input": map[string]any{"command": "cargo build --release"}, "tool_response": map[string]any{"success": true}},
		"copilot-camel": {"sessionId": "x", "cwd": "/repo", "toolName": "bash", "toolArgs": map[string]any{"command": "aws s3 ls s3://b"}, "toolResult": map[string]any{"resultType": "success"}},
		"copilot-snake": {"tool_name": "bash", "tool_input": map[string]any{"command": "gcloud auth login"}, "tool_result": map[string]any{"result_type": "success"}},
		"opencode":      {"tool_name": "bash", "tool_input": map[string]any{"command": "bun test --coverage"}, "tool_response": map[string]any{"success": true}},
		"cursor-shell":  {"command": "pnpm dlx prisma migrate dev", "exit_code": 0},
		"cursor-tool":   {"tool_name": "Shell", "tool_input": map[string]any{"command": "kubectl rollout restart deploy/api"}},
	}
	for name, payload := range cases {
		records := RecordsFromPayload(payload, name)
		if len(records) != 1 || records[0].Kind != "shell" || records[0].Signature == "" || !records[0].Success {
			t.Fatalf("%s: %+v", name, records)
		}
	}
}

func TestReferenceStringEncodedArgumentsAreParsed(t *testing.T) {
	encoded, _ := json.Marshal(map[string]any{"command": "docker ps -a"})
	records := RecordsFromPayload(map[string]any{"toolName": "bash", "toolArgs": string(encoded)}, "copilot")
	if len(records) != 1 || records[0].Binary != "docker" {
		t.Fatalf("%+v", records)
	}
}

func TestReferenceFailureShapesAreDetected(t *testing.T) {
	cases := []map[string]any{
		{"tool_name": "Bash", "tool_input": map[string]any{"command": "psql -c 'select 1'"}, "tool_response": map[string]any{"is_error": true}},
		{"hook_event_name": "PostToolUseFailure", "tool_name": "Bash", "tool_input": map[string]any{"command": "psql -c 'select 2'"}},
		{"tool_name": "bash", "tool_input": map[string]any{"command": "psql -c 'select 3'"}, "exit_code": 1},
	}
	for _, payload := range cases {
		records := RecordsFromPayload(payload, "test")
		if len(records) != 1 || records[0].Success {
			t.Fatalf("payload=%+v records=%+v", payload, records)
		}
	}
}

func TestReferenceSearchToolsBecomeRipgrep(t *testing.T) {
	claude := RecordsFromPayload(map[string]any{"tool_name": "Grep", "tool_input": map[string]any{"pattern": "TODO", "glob": "*.go", "-i": true, "-C": 2}}, "claude")
	devin := RecordsFromPayload(map[string]any{"tool_name": "grep", "tool_input": map[string]any{"pattern": "TODO", "glob_pattern": "*.go", "case_insensitive": true, "context_lines": 2}}, "devin")
	if len(claude) != 1 || len(devin) != 1 || claude[0].Binary != "rg" || claude[0].Signature != devin[0].Signature {
		t.Fatalf("claude=%+v devin=%+v", claude, devin)
	}
}

func TestReferenceEditToolsAreDocumentedOncePerKind(t *testing.T) {
	a := RecordsFromPayload(map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": "/a"}}, "claude")
	b := RecordsFromPayload(map[string]any{"tool_name": "write", "tool_input": map[string]any{"file_path": "/b"}}, "devin")
	if len(a) != 1 || len(b) != 1 || a[0].Signature != b[0].Signature || a[0].Kind != "agent-tool" {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
}

func TestReferenceUnknownPayloadIsIgnoredNotCrashed(t *testing.T) {
	if records := RecordsFromPayload(map[string]any{}, "x"); len(records) != 0 {
		t.Fatalf("%+v", records)
	}
	if records := RecordsFromPayload(map[string]any{"tool_name": "WebFetch", "tool_input": map[string]any{"url": "http://x"}}, "x"); len(records) != 0 {
		t.Fatalf("%+v", records)
	}
}

func TestPayloadNormalizationNeverRetainsArgumentValues(t *testing.T) {
	records := RecordsFromPayload(map[string]any{"tool_name": "bash", "tool_input": map[string]any{"command": "helm upgrade private-client ./secret/chart --namespace confidential"}}, "test")
	if len(records) != 1 || records[0].Command != "helm upgrade --namespace" {
		t.Fatalf("%+v", records)
	}
}
