package main

import (
	"strings"
	"testing"
)

func TestGenerateEnvContentWritesVisibleRuntimeDefaults(t *testing.T) {
	cfg := &SetupConfig{
		ProjectRoot:   "/srv/devtrack",
		WorkspacePath: "/work/project",
		DataDir:       "/data/devtrack",
		Mode:          ModeManaged,
		LLMProvider:   "ollama",
	}
	content := generateEnvContent(cfg)
	want := []string{
		"IPC_CONNECT_TIMEOUT_SECS=5",
		"HTTP_TIMEOUT_SHORT=10",
		"HTTP_TIMEOUT=30",
		"HTTP_TIMEOUT_LONG=60",
		"IPC_RETRY_DELAY_MS=2000",
		"LLM_REQUEST_TIMEOUT_SECS=120",
		"SENTIMENT_ANALYSIS_WINDOW_MINUTES=120",
		"LMSTUDIO_HOST=http://localhost:1234/v1",
		"GIT_SAGE_DEFAULT_MODEL=llama3.2",
		"PROMPT_TIMEOUT_SIMPLE_SECS=30",
		"PROMPT_TIMEOUT_WORK_SECS=60",
		"PROMPT_TIMEOUT_TASK_SECS=120",
	}
	for _, entry := range want {
		if !strings.Contains(content, "\n"+entry+"\n") {
			t.Errorf("generated environment missing visible default %q", entry)
		}
	}
}
