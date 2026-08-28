# LLM strategy

## Current strategy

DevTrack is local-first and provider-flexible:

1. Use Ollama as the normal daily provider.
2. Reuse an installed generation model instead of forcing a specific download.
3. Use explicitly configured OpenAI, Anthropic, Groq, or LM Studio providers as fallbacks.
4. Degrade to validated raw/template data when every provider is unavailable.

There is no separate `llm.yaml`, cost-limit CLI, automatic budget router, or per-feature provider
selector. Those surfaces appeared in older planning documents but were never the shipped interface.

## Privacy boundary

Local Ollama keeps inference on the configured host. A cloud provider receives the prompt text needed
for the selected generation request. Learning data remains local unless a user explicitly enables a
remote source or server sync.

## Configuration

Use `devtrack setup` and the registered environment file. The maintained variables are documented in
[Configuration](CONFIGURATION.md) and in the client/server `.env_sample` files.

## Product rule

An unavailable LLM must never block the developer's Git workflow. The daemon records the degradation,
stages what it can, and continues observing.
