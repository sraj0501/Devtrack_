package llmclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeOllamaHost(t *testing.T) {
	cases := map[string]string{
		"0.0.0.0":              "http://127.0.0.1:11434",
		"0.0.0.0:11500":        "http://127.0.0.1:11500",
		"http://0.0.0.0:11434": "http://127.0.0.1:11434",
		"localhost":            "http://localhost:11434",
	}
	for input, want := range cases {
		if got := NormalizeOllamaHost(input); got != want {
			t.Fatalf("NormalizeOllamaHost(%q)=%q want %q", input, got, want)
		}
	}
}

func TestOllamaChatJSONUsesContextAndJSONMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["format"] != "json" || request["stream"] != true {
			t.Fatalf("request=%#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"{\\\"ok\\\":\"},\"done\":false}\n"))
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"true}\"},\"done\":true}\n"))
	}))
	defer server.Close()

	cfg := Config{Host: server.URL, Model: "local", Provider: "ollama", Client: server.Client()}
	got, err := cfg.ChatJSONContext(context.Background(), []Message{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"ok":true}` {
		t.Fatalf("response=%q", got)
	}
}

func TestOpenAIChatJSONAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := Config{Host: server.URL, Model: "model", Provider: "openai", Token: "secret", Client: server.Client()}
	_, err := cfg.ChatJSONContext(ctx, []Message{{Role: "user", Content: "test"}})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("err=%v", err)
	}
}
