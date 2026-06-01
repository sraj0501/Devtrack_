---
name: Subsystem references
description: git-sage, RAG personalization, and Azure DevOps integration quick reference
type: reference
---

## git-sage

Approval modes: `[a]` auto / `[r]` review / `[s]` suggest-only. `--yes` skips. Up to 5 follow-ups after task. `undo [N]` = `git reset --hard <pre-step-HEAD>` (git state only).
Squash: `git reset --soft HEAD~N && git commit` — never `git rebase -i`.
JSON mode: Ollama `"format":"json"`; OpenAI/Groq `response_format:{"type":"json_object"}` with `BadRequestError` fallback. Strip `provider/` prefix before API call.
Groq: use `llama-3.3-70b-versatile` (better JSON than `compound-beta`). openai SDK avoids Cloudflare 403.
Env: `GIT_SAGE_PROVIDER`, `GIT_SAGE_DEFAULT_MODEL`, `GROQ_API_KEY`, `GROQ_MODEL`.

## RAG Personalization

`backend/personalization.py:inject_style(prompt, context_type, query_text)` injects two signals: style profile + ChromaDB RAG examples (`nomic-embed-text`). Setup: `ollama pull nomic-embed-text`. Config: `PERSONALIZATION_RAG_ENABLED`, `PERSONALIZATION_CHROMA_DIR`.

## Azure DevOps

Secret (.env only): `AZURE_DEVOPS_PAT`. PAT scopes: `Work Items (Read & Write)`, `Code (Read)`.
All other config lives in **workspaces.yaml** per-workspace: `pm_org` (org name), `pm_project` (project name), `pm_username` (email for assignee filter), `pm_api_url` (ADO Server URL, blank = dev.azure.com).
Server-side: `backend/config.py` → `azure_org()`, `azure_pat()` (for the Python AI pipeline; separate from the Go connector).
