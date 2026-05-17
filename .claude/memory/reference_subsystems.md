---
name: Subsystem references
description: git-sage, RAG personalization, and Azure DevOps integration quick reference
type: reference
---

## git-sage

Session approval modes: `[a]` auto / `[r]` review each / `[s]` suggest-only. `--yes`/`-y` skips dialog.
After task completes: up to 5 follow-up questions in same context. `history` / `undo [N]` available inline.
Undo: `git reset --hard <pre-step-HEAD>` — only git state, not file edits.
Squash: always `git reset --soft HEAD~N && git commit` — never `git rebase -i`.
JSON mode: Ollama uses `"format":"json"`; OpenAI/Groq uses `response_format:{"type":"json_object"}` with `BadRequestError` fallback. Model names strip `provider/` prefix before API call.
Groq: `GIT_SAGE_PROVIDER=groq` + `GROQ_API_KEY`. Use `llama-3.3-70b-versatile` (better JSON than `compound-beta`). Uses openai SDK (avoids Cloudflare 403 on urllib UA).

Key env vars: `GIT_SAGE_PROVIDER`, `GIT_SAGE_DEFAULT_MODEL`, `GIT_SAGE_API_KEY`, `GIT_SAGE_BASE_URL`, `GROQ_API_KEY`, `GROQ_HOST`, `GROQ_MODEL`.
Key files: `backend/git_sage/cli.py`, `agent.py`, `llm.py`, `config.py`.

## RAG Personalization

Two signals injected into every LLM prompt via `backend/personalization.py:inject_style(prompt, context_type, query_text)`:
1. **Profile** — `PersonalizedAI.get_style_instruction()` (formality, length, emoji pref, phrases)
2. **RAG** — ChromaDB + `nomic-embed-text`; retrieves semantically similar past responses

Injection points: `commit_message_enhancer.py` (commit), `description_enhancer.py` (description), `git_sage/agent.py` (commit), `daily_report_generator.py` (report), `ai/create_tasks.py` (task), `project_manager.py` (task/comment).

RAG modules: `backend/rag/embedder.py` (Ollama /api/embed), `vector_store.py` (ChromaDB), `sample_indexer.py` (index/retrieve).
Setup: `ollama pull nomic-embed-text`. Config: `PERSONALIZATION_RAG_ENABLED`, `PERSONALIZATION_EMBED_MODEL`, `PERSONALIZATION_RAG_K`, `PERSONALIZATION_CHROMA_DIR`.

## Azure DevOps

Required env vars: `AZURE_DEVOPS_PAT`, `AZURE_ORGANIZATION`, `AZURE_PROJECT`.
Optional: `AZURE_API_VERSION` (default 7.1), `EMAIL` (filter assigned items), `AZURE_PARENT_WORK_ITEM_ID`, `AZURE_DEFAULT_ASSIGNEE`.
PAT scopes needed: `Work Items (Read & Write)`, `Code (Read)`.
Config accessors: `backend/config.py` → `azure_org()`, `azure_pat()`. Docs: `docs/CONFIGURATION.md`.
