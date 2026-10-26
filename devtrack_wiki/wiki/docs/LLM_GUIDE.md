# LLM provider guide

Ollama is DevTrack's default provider. Cloud providers are optional fallbacks and must be configured
explicitly.

Managed mode starts a local Python server. External mode leaves server operation to you: connect a
separately running Python/PostgreSQL server with `DEVTRACK_SERVER_URL` and configure its model there.
With no reachable server, Git capture, deterministic ticket mapping, local SQLite, the pending queue,
and MCP continue; server-generated reports, ticket-comment enrichment, Boardroom, and planning are
unavailable. External mode also skips automatic first-run voice mining. On a reachable server,
model outages degrade ticket and EOD writing to raw commit text, templated comments, and bullet
lists; Boardroom and planning still need a model. The explicit
`devtrack git commit` AI helper and optional Go-side matching/Sage distillation use their own local
model configuration; normal `git commit` and Sage capture do not call a model.

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
