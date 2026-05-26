package gitsage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Message is an OpenAI-compatible chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is sent to Ollama's /api/chat endpoint.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Format   string    `json:"format,omitempty"` // "json" for structured output
}

type chatResponseChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// LLMConfig holds the provider configuration for git-sage.
type LLMConfig struct {
	Host  string // Ollama base URL, e.g. http://localhost:11434
	Model string // e.g. llama3.2
}

// LoadLLMConfig reads provider config from environment variables.
func LoadLLMConfig() LLMConfig {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("GIT_SAGE_DEFAULT_MODEL")
	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
	}
	if model == "" {
		model = "llama3.2"
	}
	return LLMConfig{Host: host, Model: model}
}

// Chat sends messages to the LLM and returns the complete response text.
func (cfg LLMConfig) Chat(messages []Message) (string, error) {
	return cfg.chat(messages, false)
}

// ChatJSON sends messages requesting structured JSON output.
func (cfg LLMConfig) ChatJSON(messages []Message) (string, error) {
	return cfg.chat(messages, true)
}

func (cfg LLMConfig) chat(messages []Message, jsonMode bool) (string, error) {
	req := chatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   true,
	}
	if jsonMode {
		req.Format = "json"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(cfg.Host, "/") + "/api/chat"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama unreachable at %s: %w\nMake sure Ollama is running: ollama serve", cfg.Host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(raw))
	}

	var result strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk chatResponseChunk
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		result.WriteString(chunk.Message.Content)
		if chunk.Done {
			break
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// Ping checks whether the Ollama server is reachable.
func (cfg LLMConfig) Ping() bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(strings.TrimRight(cfg.Host, "/") + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
