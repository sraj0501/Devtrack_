---
name: RAG Personalization Reference
description: Two-signal personalization system: profile-based + RAG few-shot examples
type: reference
---

## Two-Signal Personalization (completed March 13, 2026)

Injected into EVERY LLM prompt in the system via `backend/personalization.py:inject_style(prompt, context_type, query_text)`.

### Signal 1: Profile-based style instruction
- Source: `PersonalizedAI.get_style_instruction()`
- Fast, always available once a profile exists
- Captures: formality, length, emoji preference, common phrases
- Output: `[STYLE: clear and direct, concise but complete, no emojis. ...]`

### Signal 2: RAG few-shot examples
- Source: `backend/rag/` modules
- ChromaDB vector store + `nomic-embed-text` via Ollama
- Retrieves semantically similar past responses the user wrote
- Output: `Here are real examples of how this user has written...`

## RAG Modules (`backend/rag/`)

| Module | Purpose |
|---|---|
| `embedder.py` | Ollama `/api/embed`, returns None if model unavailable |
| `vector_store.py` | ChromaDB PersistentClient, cosine similarity, context_type filter |
| `sample_indexer.py` | `index_sample()`, `index_samples()`, `retrieve_examples()` |

## Injection Points (all 6 output generators wired)

| File | context_type | RAG query |
|---|---|---|
| `commit_message_enhancer.py` | `commit` | original message |
| `description_enhancer.py` | `description` | raw_input |
| `git_sage/agent.py` | `commit` | appended to system prompt |
| `daily_report_generator.py` | `report` | first 200 chars |
| `ai/create_tasks.py` | `task` | first 200 chars |
| `project_manager.py` | `task`/`comment` | first 200 chars |

## Auto-Indexing

- New samples indexed immediately in `add_communication_sample()`
- Profile load triggers incremental index
- Revoke consent wipes ChromaDB

## Setup

```bash
ollama pull nomic-embed-text    # One-time
# ChromaDB installed via: uv sync
```

## Config Vars (all optional with defaults)

- `PERSONALIZATION_RAG_ENABLED=true`
- `PERSONALIZATION_EMBED_MODEL=nomic-embed-text`
- `PERSONALIZATION_RAG_K=3`
- `PERSONALIZATION_CHROMA_DIR=${DATA_DIR}/learning/chroma`

## Key Files

```
backend/
  personalization.py       - Global inject_style() — combines profile + RAG
  personalized_ai.py       - Talk Like You AI engine + get_style_instruction()
  learning_integration.py  - Teams collection, GraphClientAdapter, AsyncTeamsDataCollector
  run_daily_learning.py    - Cron script for daily delta sync
  rag/
    __init__.py            - Package entry: get_indexer()
    embedder.py            - Ollama /api/embed calls
    vector_store.py        - ChromaDB PersistentClient wrapper
    sample_indexer.py      - index_sample/index_samples/retrieve_examples
  llm/
    provider_factory.py    - Multi-provider fallback chain (includes Groq)
    groq_provider.py       - Groq via openai SDK
```
