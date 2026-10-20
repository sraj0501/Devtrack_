package gitcmd

import (
	"os"
	"strings"

	devconfig "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// Provider constants for the shared LLM provider setting.
const (
	ProviderOllama = "ollama"
	ProviderOpenAI = "openai"
	ProviderGroq   = "groq"
)

// Config holds commit-enhancement configuration derived from shared settings.
type Config struct {
	LLM      LLMConfig
	Provider string
}

// LoadConfig builds commit-enhancement configuration. Ollama remains the
// offline-first default; legacy Git Sage variables are intentionally ignored.
func LoadConfig() Config {
	provider := devconfig.GetLLMProvider()

	llm := buildLLMConfig(provider)

	return Config{
		LLM:      llm,
		Provider: provider,
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
//
//	OPENAI_API_KEY    — required for OpenAI; use any string for Groq/LM Studio
//	OPENAI_BASE_URL   — base URL (default: https://api.openai.com/v1)
//	GROQ_API_KEY      — used when provider=groq (overrides OPENAI_API_KEY)
//	GROQ_HOST         — base URL override for Groq
//	OPENAI_MODEL / GROQ_MODEL — model name
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
		model = os.Getenv("OPENAI_MODEL")
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
