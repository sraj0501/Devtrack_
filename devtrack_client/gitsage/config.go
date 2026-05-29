package gitsage

import (
	"os"
	"strings"
)

// Provider constants for SAGE_PROVIDER.
const (
	ProviderOllama = "ollama"
	ProviderOpenAI = "openai"
	ProviderGroq   = "groq"
)

// Config holds all git-sage configuration derived from environment variables.
type Config struct {
	LLM       LLMConfig
	Provider  string
	MaxSteps  int
	Verbose   bool
	SuggestOnly bool
}

// LoadConfig builds the full git-sage Config from environment variables.
// Safe defaults are provided for model and host; no panic on missing vars.
func LoadConfig() Config {
	provider := strings.ToLower(os.Getenv("SAGE_PROVIDER"))
	if provider == "" {
		provider = strings.ToLower(os.Getenv("GIT_SAGE_PROVIDER"))
	}
	if provider == "" {
		provider = ProviderOllama
	}

	llm := buildLLMConfig(provider)

	return Config{
		LLM:         llm,
		Provider:    provider,
		MaxSteps:    maxSteps,
		Verbose:     os.Getenv("SAGE_VERBOSE") == "1" || os.Getenv("SAGE_VERBOSE") == "true",
		SuggestOnly: os.Getenv("SAGE_SUGGEST_ONLY") == "1",
	}
}

// buildLLMConfig constructs an LLMConfig for the given provider.
// Extends the base LoadLLMConfig to support OpenAI-compatible endpoints
// (OpenAI, Groq, LM Studio, etc.) via OPENAI_BASE_URL.
func buildLLMConfig(provider string) LLMConfig {
	switch provider {
	case ProviderOpenAI, ProviderGroq:
		return buildOpenAICompatConfig(provider)
	default:
		// Ollama or unknown — use existing loader
		return LoadLLMConfig()
	}
}

// buildOpenAICompatConfig constructs an LLMConfig for OpenAI-compatible endpoints.
// Env vars:
//   OPENAI_API_KEY    — required for OpenAI; use any string for Groq/LM Studio
//   OPENAI_BASE_URL   — base URL (default: https://api.openai.com/v1)
//   GROQ_API_KEY      — used when provider=groq (overrides OPENAI_API_KEY)
//   GROQ_HOST         — base URL override for Groq
//   SAGE_MODEL / GIT_SAGE_DEFAULT_MODEL — model name
func buildOpenAICompatConfig(provider string) LLMConfig {
	var host, token, model string

	switch provider {
	case ProviderGroq:
		token = os.Getenv("GROQ_API_KEY")
		host = os.Getenv("GROQ_HOST")
		if host == "" {
			host = "https://api.groq.com/openai/v1"
		}
		model = os.Getenv("GROQ_MODEL")
	default: // openai
		token = os.Getenv("OPENAI_API_KEY")
		host = os.Getenv("OPENAI_BASE_URL")
		if host == "" {
			host = "https://api.openai.com/v1"
		}
	}

	if model == "" {
		model = os.Getenv("SAGE_MODEL")
	}
	if model == "" {
		model = os.Getenv("GIT_SAGE_DEFAULT_MODEL")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	// Strip provider/ prefix (LiteLLM convention)
	if idx := strings.Index(model, "/"); idx >= 0 {
		model = model[idx+1:]
	}

	return LLMConfig{
		Host:     strings.TrimRight(host, "/"),
		Model:    model,
		Token:    token,
		Provider: provider,
	}
}

// SageModelName returns the model name to use, stripping provider/ prefix.
func SageModelName() string {
	m := os.Getenv("SAGE_MODEL")
	if m == "" {
		m = os.Getenv("GIT_SAGE_DEFAULT_MODEL")
	}
	if m == "" {
		m = os.Getenv("OLLAMA_MODEL")
	}
	if m == "" {
		m = "llama3.2"
	}
	if idx := strings.Index(m, "/"); idx >= 0 {
		m = m[idx+1:]
	}
	return m
}
