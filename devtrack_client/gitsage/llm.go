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
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   string         `json:"format,omitempty"` // "json" for structured output
	Options  *ollamaOptions `json:"options,omitempty"`
}

// ollamaOptions carries generation tunables for the Ollama /api/chat endpoint.
type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"` // max tokens to generate; 0 = model default
}

type chatResponseChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// LLMConfig holds the provider configuration for git-sage.
type LLMConfig struct {
	Host     string // base URL: Ollama or OpenAI-compatible endpoint
	Model    string // model name (provider/ prefix already stripped)
	Token    string // API key for OpenAI-compatible providers; empty for Ollama
	Provider string // "ollama" | "openai" | "groq" | "lmstudio"
}

// LoadLLMConfig reads provider config from environment variables (Ollama default).
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
	return LLMConfig{Host: host, Model: model, Provider: "ollama"}
}

// Chat sends messages to the LLM and returns the complete response text.
func (cfg LLMConfig) Chat(messages []Message) (string, error) {
	return cfg.chat(messages, false, 0)
}

// ChatJSON sends messages requesting structured JSON output.
func (cfg LLMConfig) ChatJSON(messages []Message) (string, error) {
	return cfg.chat(messages, true, 0)
}

// ChatWithTokens sends messages with an explicit max-token budget (0 = model default).
func (cfg LLMConfig) ChatWithTokens(messages []Message, maxTokens int) (string, error) {
	return cfg.chat(messages, false, maxTokens)
}

func (cfg LLMConfig) chat(messages []Message, jsonMode bool, maxTokens int) (string, error) {
	if cfg.Provider == "openai" || cfg.Provider == "groq" || cfg.Provider == "lmstudio" {
		return cfg.chatOpenAI(messages, jsonMode, maxTokens)
	}
	return cfg.chatOllama(messages, jsonMode, maxTokens)
}

// chatOllama calls the Ollama /api/chat streaming endpoint.
func (cfg LLMConfig) chatOllama(messages []Message, jsonMode bool, maxTokens int) (string, error) {
	req := chatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   true,
	}
	if jsonMode {
		req.Format = "json"
	}
	if maxTokens > 0 {
		req.Options = &ollamaOptions{NumPredict: maxTokens}
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
			break
		}
		result.WriteString(chunk.Message.Content)
		if chunk.Done {
			break
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// openAIRequest is the request body for OpenAI-compatible /v1/chat/completions.
type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Stream         bool            `json:"stream"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// chatOpenAI calls an OpenAI-compatible /v1/chat/completions endpoint.
// Supports OpenAI, Groq, LM Studio, and any other OpenAI-compatible provider.
func (cfg LLMConfig) chatOpenAI(messages []Message, jsonMode bool, maxTokens int) (string, error) {
	req := openAIRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   false, // non-streaming for simplicity; easier to parse
	}
	if jsonMode {
		req.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	if maxTokens > 0 {
		req.MaxTokens = maxTokens
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(cfg.Host, "/") + "/chat/completions"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%s unreachable at %s: %w", cfg.Provider, cfg.Host, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned %d: %s", cfg.Provider, resp.StatusCode, string(raw))
	}

	var result openAIResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("openai response parse: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai returned empty choices")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

// Ping checks whether the LLM provider is reachable.
// OpenAI-compatible providers check /models; Ollama checks /api/tags.
func (cfg LLMConfig) Ping() bool {
	client := &http.Client{Timeout: 3 * time.Second}
	var checkURL string
	if cfg.Provider == "openai" || cfg.Provider == "groq" || cfg.Provider == "lmstudio" {
		checkURL = strings.TrimRight(cfg.Host, "/") + "/models"
	} else {
		checkURL = strings.TrimRight(cfg.Host, "/") + "/api/tags"
	}
	req, err := http.NewRequest(http.MethodGet, checkURL, nil)
	if err != nil {
		return false
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
