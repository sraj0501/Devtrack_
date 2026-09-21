package gitcmd

import "github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"

// The explicit Git helper retains its public types while the provider
// transport lives in a neutral package shared with background Sage workers.
type Message = llmclient.Message
type LLMConfig = llmclient.Config

func LoadLLMConfig() LLMConfig {
	return llmclient.LoadOllamaConfig()
}
