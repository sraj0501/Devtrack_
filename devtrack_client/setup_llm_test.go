package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetectUsableOllamaModelSkipsEmbeddingModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("path = %q, want /api/tags", r.URL.Path)
		}
		fmt.Fprint(w, `{"models":[{"name":"nomic-embed-text:latest","details":{"family":"bert"}},{"name":"qwen2.5:7b","details":{"family":"qwen2"}}]}`)
	}))
	defer server.Close()

	model, err := detectUsableOllamaModel(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if model != "qwen2.5:7b" {
		t.Fatalf("model = %q, want qwen2.5:7b", model)
	}
}

func TestDetectUsableOllamaModelAcceptsModelField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"models":[{"model":"llama3.1:8b"}]}`)
	}))
	defer server.Close()

	model, err := detectUsableOllamaModel(strings.TrimPrefix(server.URL, "http://"), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if model != "llama3.1:8b" {
		t.Fatalf("model = %q, want llama3.1:8b", model)
	}
}

func TestNormalizeSetupOllamaHostMakesBindAddressConnectable(t *testing.T) {
	for raw, want := range map[string]string{
		"localhost":         "http://localhost:11434",
		"0.0.0.0:11434":     "http://localhost:11434",
		"http://host.test/": "http://host.test:11434",
	} {
		got, err := normalizeSetupOllamaHost(raw)
		if err != nil {
			t.Fatalf("normalizeSetupOllamaHost(%q): %v", raw, err)
		}
		if got != want {
			t.Errorf("normalizeSetupOllamaHost(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestConfigureOllamaSetupReusesDetectedModel(t *testing.T) {
	cfg := &SetupConfig{}
	configureOllamaSetup(bufio.NewReader(strings.NewReader("\n")), cfg, "http://ollama.test:11434", "mistral:7b")
	if cfg.LLMProvider != "ollama" || cfg.OllamaHost != "http://ollama.test:11434" || cfg.OllamaModel != "mistral:7b" {
		t.Fatalf("unexpected setup config: %+v", cfg)
	}
	if !cfg.OllamaModelReady {
		t.Fatal("detected model was not marked ready")
	}
}

func TestConfigureOllamaSetupAcceptsCloudFastLane(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-secret")
	cfg := &SetupConfig{}
	reader := bufio.NewReader(strings.NewReader("\n\n\n"))
	configureOllamaSetup(reader, cfg, "http://ollama.test:11434", "")
	if cfg.LLMProvider != "ollama" || cfg.OllamaModel != "llama3.2" {
		t.Fatalf("Ollama was not kept primary: %+v", cfg)
	}
	if cfg.OpenAIKey != "openai-secret" || cfg.AnthropicKey != "anthropic-secret" {
		t.Fatalf("cloud fast-lane keys were not retained: %+v", cfg)
	}
}

func TestOfferCloudFastLaneCanBeDeclined(t *testing.T) {
	cfg := &SetupConfig{LLMProvider: "ollama"}
	offerCloudFastLane(bufio.NewReader(strings.NewReader("n\n")), cfg, setupCloudKeys{OpenAI: "secret"})
	if cfg.OpenAIKey != "" {
		t.Fatal("declined cloud key was retained")
	}
}
