# LLM provider guide

Ollama is DevTrack's default provider. Cloud providers are optional fallbacks and must be configured
explicitly.

## Ollama

```bash
LLM_PROVIDER=ollama
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=<installed-generation-model>
```

Check local availability:

```bash
curl http://localhost:11434/api/tags
devtrack doctor
```

Setup reuses a compatible installed generation model and skips a redundant pull. Embedding-only
models are not selected for generation.

## Optional providers

```bash
OPENAI_API_KEY=<secret>
OPENAI_MODEL=gpt-4o-mini

ANTHROPIC_API_KEY=<secret>
ANTHROPIC_MODEL=claude-haiku-4-5

GROQ_API_KEY=<secret>
GROQ_MODEL=llama-3.3-70b-versatile
```

LM Studio uses `LMSTUDIO_HOST`. `OLLAMA_URL` and `LMSTUDIO_URL` are obsolete configuration names.

When setup finds an existing OpenAI or Anthropic key but no usable local model, it may offer that key
as an explicit temporary fast lane. Ollama remains the primary provider and resumes local generation
when ready.

## Failure behavior

Provider failures do not block Git. Structured task enrichment falls back to raw commit data;
generation falls back through configured providers and ultimately to template/raw output. Use
`devtrack status`, `devtrack doctor`, and `devtrack logs -f` to diagnose readiness.
