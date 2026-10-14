package distill

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
)

func installed(path string) func(string) (string, error) {
	return func(string) (string, error) { return path, nil }
}

func TestReferencePlaceholdersAreRenderedAndEmptyModelDropped(t *testing.T) {
	knowledgeRoot := filepath.Join(t.TempDir(), "personal knowledge")
	specs := map[string]BackendSpec{"fake": {
		Binary: "fake", Model: "", Prompt: PromptArg,
		Args: []string{"-m", "{model}", "--cd", "{kb}", "{prompt_text}"},
	}}
	backend, found := ResolveBackend([]string{"fake"}, specs, knowledgeRoot, installed("/tools/fake"))
	if !found {
		t.Fatal("installed backend was not resolved")
	}
	if slices.Contains(backend.Args, "-m") || slices.Contains(backend.Args, "{model}") {
		t.Fatalf("empty model flag/value remained in %q", backend.Args)
	}
	if !slices.Contains(backend.Args, knowledgeRoot) {
		t.Fatalf("knowledge placeholder was not rendered in %q", backend.Args)
	}
	if !slices.Contains(backend.Args, "{prompt_text}") {
		t.Fatalf("request prompt was resolved too early in %q", backend.Args)
	}
}

func TestReferenceNoBackendWhenBinaryMissing(t *testing.T) {
	specs := map[string]BackendSpec{"missing": {
		Binary: "definitely-not-installed-xyz", Prompt: PromptStdin,
	}}
	lookPath := func(string) (string, error) { return "", errors.New("missing") }
	if backend, found := ResolveBackend([]string{"missing"}, specs, t.TempDir(), lookPath); found || backend != nil {
		t.Fatalf("missing executable resolved as %#v", backend)
	}
}

func TestReferenceShippedBackendsDeclareAPromptMode(t *testing.T) {
	for name, spec := range ShippedTextBackends() {
		if !validPromptMode(spec.Prompt) {
			t.Fatalf("%s prompt mode = %q", name, spec.Prompt)
		}
		if spec.Prompt == PromptArg && !slices.Contains(spec.Args, "{prompt_text}") {
			t.Fatalf("%s arg backend has no prompt placeholder", name)
		}
		if spec.Prompt == PromptFile && !slices.Contains(spec.Args, "{prompt_file}") {
			t.Fatalf("%s file backend has no prompt placeholder", name)
		}
	}
}

func TestReferenceTextBackendDefersPromptFileSubstitution(t *testing.T) {
	runtimeRoot := t.TempDir()
	specs := map[string]BackendSpec{"fake": {
		Binary: "fake", Model: "m", Prompt: PromptFile,
		Args: []string{"--prompt-file", "{prompt_file}"},
	}}
	backend, found := ResolveBackend([]string{"fake"}, specs, t.TempDir(), installed("fake"))
	if !found || !slices.Contains(backend.Args, "{prompt_file}") {
		t.Fatalf("prompt file was resolved before a request: %#v", backend)
	}
	backend.RuntimeRoot = runtimeRoot
	var usedPath string
	backend.run = func(_ context.Context, _ string, args []string, stdin string) (string, error) {
		if stdin != "" || len(args) != 2 || args[0] != "--prompt-file" {
			t.Fatalf("args=%q stdin=%q", args, stdin)
		}
		usedPath = args[1]
		body, err := os.ReadFile(usedPath)
		if err != nil {
			return "", err
		}
		if !strings.Contains(string(body), "system:\nReturn JSON") || !strings.Contains(string(body), "user:\nCaptured fact") {
			t.Fatalf("unexpected prompt: %q", body)
		}
		return `{"worth_documenting":false,"skip_reason":"test"}`, nil
	}
	if _, err := backend.ChatJSONContext(context.Background(), []llmclient.Message{
		{Role: "system", Content: "Return JSON"}, {Role: "user", Content: "Captured fact"},
	}); err != nil {
		t.Fatal(err)
	}
	if usedPath == "" || filepath.Dir(usedPath) != runtimeRoot {
		t.Fatalf("per-call prompt path = %q", usedPath)
	}
	if _, err := os.Stat(usedPath); !os.IsNotExist(err) {
		t.Fatalf("temporary prompt was not removed: %v", err)
	}
}

func TestLoadModelClientKeepsOllamaAsDefault(t *testing.T) {
	t.Setenv("DEVTRACK_SAGE_MODEL_BACKEND", "")
	client, err := LoadModelClient(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := client.(llmclient.Config); !ok {
		t.Fatalf("default client = %T, want Ollama HTTP config", client)
	}
}
