// Package llmclient provides a context-aware transport for background model
// work. Product packages own prompts, validation, and retry policy.
package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxErrorBody = 4096

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Config struct {
	Host     string
	Model    string
	Token    string
	Provider string
	Client   *http.Client
	Timeout  time.Duration
}

func LoadOllamaConfig() Config {
	host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if host == "" {
		host = "http://localhost:11434"
	}
	model := strings.TrimSpace(os.Getenv("OLLAMA_MODEL"))
	if model == "" {
		model = "llama3.2"
	}
	return Config{Host: NormalizeOllamaHost(host), Model: model, Provider: "ollama"}
}

func NormalizeOllamaHost(host string) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.Replace(host, "//0.0.0.0", "//127.0.0.1", 1)
	}
	if strings.HasPrefix(host, "0.0.0.0") {
		host = "127.0.0.1" + host[7:]
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	withoutScheme := host[strings.Index(host, "://")+3:]
	if !strings.Contains(withoutScheme, ":") {
		host += ":11434"
	}
	return host
}

func (cfg Config) client() *http.Client {
	if cfg.Client != nil {
		return cfg.Client
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

func (cfg Config) openAICompatible() bool {
	return cfg.Provider == "openai" || cfg.Provider == "groq" || cfg.Provider == "lmstudio"
}

func (cfg Config) Chat(messages []Message) (string, error) {
	return cfg.chat(context.Background(), messages, false, 0)
}

func (cfg Config) ChatJSON(messages []Message) (string, error) {
	return cfg.chat(context.Background(), messages, true, 0)
}

func (cfg Config) ChatWithTokens(messages []Message, maxTokens int) (string, error) {
	return cfg.chat(context.Background(), messages, false, maxTokens)
}

// ChatJSONContext performs cancellable background generation. Callers must
// never use this from a capture hook or another latency-sensitive operation.
func (cfg Config) ChatJSONContext(ctx context.Context, messages []Message) (string, error) {
	return cfg.chat(ctx, messages, true, 0)
}

func (cfg Config) chat(ctx context.Context, messages []Message, jsonMode bool, maxTokens int) (string, error) {
	if cfg.openAICompatible() {
		return cfg.chatOpenAI(ctx, messages, jsonMode, maxTokens)
	}
	return cfg.chatOllama(ctx, messages, jsonMode, maxTokens)
}

type ollamaRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   string         `json:"format,omitempty"`
	Options  *ollamaOptions `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

type ollamaChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (cfg Config) chatOllama(ctx context.Context, messages []Message, jsonMode bool, maxTokens int) (string, error) {
	payload := ollamaRequest{Model: cfg.Model, Messages: messages, Stream: true}
	if jsonMode {
		payload.Format = "json"
	}
	if maxTokens > 0 {
		payload.Options = &ollamaOptions{NumPredict: maxTokens}
	}
	request, err := cfg.request(ctx, strings.TrimRight(cfg.Host, "/")+"/api/chat", payload)
	if err != nil {
		return "", err
	}
	response, err := cfg.client().Do(request)
	if err != nil {
		return "", fmt.Errorf("ollama model request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", responseError("ollama", response)
	}
	var result strings.Builder
	decoder := json.NewDecoder(response.Body)
	for {
		var chunk ollamaChunk
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("ollama response parse: %w", err)
		}
		result.WriteString(chunk.Message.Content)
		if chunk.Done {
			break
		}
	}
	return nonempty("ollama", result.String())
}

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
	} `json:"choices"`
}

func (cfg Config) chatOpenAI(ctx context.Context, messages []Message, jsonMode bool, maxTokens int) (string, error) {
	payload := openAIRequest{Model: cfg.Model, Messages: messages}
	if jsonMode {
		payload.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	if maxTokens > 0 {
		payload.MaxTokens = maxTokens
	}
	request, err := cfg.request(ctx, strings.TrimRight(cfg.Host, "/")+"/chat/completions", payload)
	if err != nil {
		return "", err
	}
	if cfg.Token != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	response, err := cfg.client().Do(request)
	if err != nil {
		return "", fmt.Errorf("%s model request failed: %w", cfg.Provider, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", responseError(cfg.Provider, response)
	}
	var result openAIResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("openai-compatible response parse: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("%s returned empty choices", cfg.Provider)
	}
	return nonempty(cfg.Provider, result.Choices[0].Message.Content)
}

func (cfg Config) request(ctx context.Context, url string, payload any) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func nonempty(provider, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s returned an empty response", provider)
	}
	return value, nil
}

func responseError(provider string, response *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	detail := strings.TrimSpace(string(raw))
	if detail == "" {
		return fmt.Errorf("%s returned HTTP %d", provider, response.StatusCode)
	}
	return fmt.Errorf("%s returned HTTP %d: %s", provider, response.StatusCode, detail)
}

func (cfg Config) PingURL() string {
	if cfg.openAICompatible() {
		return strings.TrimRight(cfg.Host, "/") + "/models"
	}
	return strings.TrimRight(cfg.Host, "/") + "/api/tags"
}

func (cfg Config) Ping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.PingURL(), nil)
	if err != nil {
		return false
	}
	if cfg.Token != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	response, err := cfg.client().Do(request)
	if err != nil {
		return false
	}
	response.Body.Close()
	return response.StatusCode == http.StatusOK
}
