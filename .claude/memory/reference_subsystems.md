---
name: Subsystem references
description: gitsage (Go-native), RAG personalization, Azure DevOps config
type: reference
---

## gitsage (`devtrack_client/gitsage/`) — Go-native

Go modules: `agent.go`, `cli.go`, `commit.go`, `config.go`, `conflict.go`, `context.go`, `git_ops.go`, `llm.go`, `pr_finder.go`.
Approval: `[a]` auto / `[r]` review / `[s]` suggest-only. `--yes` skips. Up to 5 follow-ups. `undo [N]` = `git reset --hard <pre-step-HEAD>`.
Squash: `git reset --soft HEAD~N && git commit` — never `git rebase -i`.
JSON mode: Ollama `"format":"json"`; OpenAI/Groq `response_format:{"type":"json_object"}` with `BadRequestError` fallback. Strip `provider/` prefix.
Groq: prefer `llama-3.3-70b-versatile` over `compound-beta` (better JSON). openai SDK avoids Cloudflare 403.

## RAG Personalization (server-side)

`devtrack_server/backend/personalization.py:inject_style(prompt, context_type, query_text)` — style profile + ChromaDB RAG (`nomic-embed-text`). Setup: `ollama pull nomic-embed-text`. Env: `PERSONALIZATION_RAG_ENABLED`, `PERSONALIZATION_CHROMA_DIR`.

## Azure DevOps

`.env` secret only: `AZURE_DEVOPS_PAT` (scopes: Work Items R/W, Code R).
`workspaces.yaml` holds all non-secret config: `pm_org`, `pm_project`, `pm_username`, `pm_api_url` (blank = dev.azure.com).
