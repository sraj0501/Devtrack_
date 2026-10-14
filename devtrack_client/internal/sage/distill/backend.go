package distill

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
)

// PromptMode describes how a command-backed model receives one complete
// prompt. Sage never invokes a shell; these modes only control argv/stdin.
type PromptMode string

const (
	PromptFile  PromptMode = "file"
	PromptArg   PromptMode = "arg"
	PromptStdin PromptMode = "stdin"
)

// BackendSpec is an argv template for a text-in/text-out model CLI.
type BackendSpec struct {
	Binary string
	Model  string
	Args   []string
	Prompt PromptMode
}

// CommandBackend is a resolved executable plus its still-late-bound prompt
// template. Prompt text and prompt-file paths are substituted per request.
type CommandBackend struct {
	Name          string
	Binary        string
	Model         string
	Args          []string
	Prompt        PromptMode
	RuntimeRoot   string
	KnowledgeRoot string
	run           commandRunner
}

type commandRunner func(context.Context, string, []string, string) (string, error)

// ShippedTextBackends returns a fresh copy of every explicitly selectable
// command provider. Ollama remains the default and uses the HTTP transport.
func ShippedTextBackends() map[string]BackendSpec {
	return map[string]BackendSpec{
		"claude": {
			Binary: "claude", Model: "haiku",
			Args: []string{"-p", "--model", "{model}"}, Prompt: PromptStdin,
		},
		"devin": {
			Binary: "devin", Model: "swe-1-7-medium",
			Args: []string{"-p", "--model", "{model}", "--respect-workspace-trust", "false", "--prompt-file", "{prompt_file}"}, Prompt: PromptFile,
		},
		"codex": {
			Binary: "codex",
			Args:   []string{"exec", "--sandbox", "read-only", "{prompt_text}"}, Prompt: PromptArg,
		},
		"copilot": {
			Binary: "copilot",
			Args:   []string{"-p", "{prompt_text}"}, Prompt: PromptArg,
		},
		"cursor-agent": {
			Binary: "cursor-agent",
			Args:   []string{"-p", "{prompt_text}"}, Prompt: PromptArg,
		},
	}
}

// ResolveBackend selects the first valid installed backend and resolves only
// stable placeholders. Request-specific prompt placeholders deliberately
// remain in Args until ChatJSONContext executes.
func ResolveBackend(order []string, specs map[string]BackendSpec, knowledgeRoot string, lookPath func(string) (string, error)) (*CommandBackend, bool) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	for _, name := range order {
		spec, found := specs[name]
		if !found || !validPromptMode(spec.Prompt) {
			continue
		}
		binaryName := strings.TrimSpace(spec.Binary)
		if binaryName == "" {
			binaryName = name
		}
		binary, err := lookPath(binaryName)
		if err != nil || strings.TrimSpace(binary) == "" {
			continue
		}
		args := make([]string, 0, len(spec.Args))
		for _, raw := range spec.Args {
			token := raw
			if token == "{model}" && strings.TrimSpace(spec.Model) == "" {
				if len(args) > 0 && (args[len(args)-1] == "--model" || args[len(args)-1] == "-m") {
					args = args[:len(args)-1]
				}
				continue
			}
			token = strings.ReplaceAll(token, "{model}", spec.Model)
			token = strings.ReplaceAll(token, "{kb}", knowledgeRoot)
			args = append(args, token)
		}
		return &CommandBackend{
			Name: name, Binary: binary, Model: spec.Model, Args: args,
			Prompt: spec.Prompt, KnowledgeRoot: knowledgeRoot,
		}, true
	}
	return nil, false
}

func validPromptMode(mode PromptMode) bool {
	return mode == PromptFile || mode == PromptArg || mode == PromptStdin
}

// LoadModelClient preserves offline-first Ollama HTTP as the default. A CLI
// provider is used only when DEVTRACK_SAGE_MODEL_BACKEND names it explicitly.
func LoadModelClient(runtimeRoot, knowledgeRoot string) (Client, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("DEVTRACK_SAGE_MODEL_BACKEND")))
	if name == "" || name == "ollama" {
		return llmclient.LoadOllamaConfig(), nil
	}
	specs := ShippedTextBackends()
	spec, found := specs[name]
	if !found {
		return nil, fmt.Errorf("unsupported Sage model backend %q", name)
	}
	if model := strings.TrimSpace(os.Getenv("DEVTRACK_SAGE_MODEL")); model != "" {
		spec.Model = model
		specs[name] = spec
	}
	backend, found := ResolveBackend([]string{name}, specs, knowledgeRoot, nil)
	if !found {
		return nil, fmt.Errorf("Sage model backend %q is not installed", name)
	}
	backend.RuntimeRoot = runtimeRoot
	return backend, nil
}

// ChatJSONContext executes one model request without a shell. The caller owns
// JSON validation and retry policy.
func (b *CommandBackend) ChatJSONContext(ctx context.Context, messages []llmclient.Message) (string, error) {
	if b == nil || strings.TrimSpace(b.Binary) == "" {
		return "", errors.New("Sage command model backend is unavailable")
	}
	prompt := renderMessages(messages)
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("Sage command model prompt is empty")
	}
	args := append([]string(nil), b.Args...)
	stdin := ""
	promptFile := ""
	if b.Prompt == PromptFile {
		root := b.RuntimeRoot
		if strings.TrimSpace(root) == "" {
			return "", errors.New("Sage command model runtime root is unavailable")
		}
		if err := os.MkdirAll(root, 0700); err != nil {
			return "", fmt.Errorf("prepare Sage command model runtime: %w", err)
		}
		file, err := os.CreateTemp(root, "model-prompt-*.md")
		if err != nil {
			return "", fmt.Errorf("create Sage command model prompt: %w", err)
		}
		promptFile = file.Name()
		defer os.Remove(promptFile)
		if _, err := file.WriteString(prompt); err != nil {
			file.Close()
			return "", fmt.Errorf("write Sage command model prompt: %w", err)
		}
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("close Sage command model prompt: %w", err)
		}
	}
	for index, arg := range args {
		args[index] = strings.ReplaceAll(arg, "{prompt_text}", prompt)
		args[index] = strings.ReplaceAll(args[index], "{prompt_file}", promptFile)
	}
	if b.Prompt == PromptStdin {
		stdin = prompt
	}
	runner := b.run
	if runner == nil {
		runner = runCommand
	}
	output, err := runner(ctx, b.Binary, args, stdin)
	if err != nil {
		return "", err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("Sage model backend %q returned an empty response", b.Name)
	}
	return output, nil
}

func renderMessages(messages []llmclient.Message) string {
	var prompt strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		prompt.WriteString(role)
		prompt.WriteString(":\n")
		prompt.WriteString(content)
		prompt.WriteString("\n\n")
	}
	return strings.TrimSpace(prompt.String())
}

func runCommand(ctx context.Context, binary string, args []string, stdin string) (string, error) {
	command := exec.CommandContext(ctx, binary, args...)
	configureBackgroundCommand(command)
	command.Env = append(os.Environ(), "DEVTRACK_SAGE_DISTILLER=1", "DEVTRACK_SAGE_CAPTURE_OFF=1")
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("Sage model command %q failed: %w", filepath.Base(binary), err)
	}
	return stdout.String(), nil
}
