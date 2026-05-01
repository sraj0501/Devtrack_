---
name: runtime-narrative integration plan
description: Feasibility assessment and integration plan for the runtime-narrative Python library
type: project
---

# runtime-narrative Integration

**Library**: `runtime-narrative` v0.1.0 (Beta, MIT, March 21, 2026)
**PyPI**: https://pypi.org/project/runtime-narrative/
**GitHub**: https://github.com/sraj0501/runtime_narrative (private/inaccessible as of March 22, 2026)

## What It Does

Structured observability: replaces scattered log lines with named **stories** (logical operations) and **stages** (sequential steps). On failure, produces a structured report showing which stage broke + optional LLM-powered self-diagnosis.

```python
with story("Commit Trigger"):
    with stage("NLP Parse"):      parsed = nlp.parse(message)
    with stage("AI Enhancement"): enhanced = llm.generate(prompt)
    with stage("Azure Sync"):     await client.add_comment(...)
```

Key features:
- Custom renderers (lifecycle event hooks)
- Structured JSON output (newline-delimited, for log aggregators)
- FastAPI middleware (`RuntimeNarrativeMiddleware`) — auto-story per HTTP request
- LLM failure analysis — OpenAI-compatible or Ollama; fires on stage failure

## Fit with DevTrack

Strong — DevTrack's pipeline architecture maps directly to story/stage. No dep conflicts (openai + ollama already in pyproject.toml).

## Integration Points (Priority Order)

1. **`python_bridge.py`** — `handle_commit_trigger()` + `handle_timer_trigger()` — highest value
2. **`backend/webhook_server.py`** — `app.add_middleware(RuntimeNarrativeMiddleware)` — one line
3. **`backend/daily_report_generator.py`** — LLM failure diagnosis particularly useful here
4. **`backend/commit_message_enhancer.py`** — per-attempt refinement tracing
5. **`backend/project_manager.py`** — cross-platform PM sync stages

## Blocker: Async Compatibility

DevTrack is asyncio-driven throughout. Must verify before any integration:

```python
async with story("test"):
    async with stage("step"):
        await asyncio.sleep(0)
```

If sync-only: write thin async wrapper in `backend/utils/narrative.py`.

**Why:** `handle_commit_trigger` and sub-calls (`AzureDevOpsClient`, `GitHubClient`, `GitLabClient`) are all async coroutines. Sync-only context managers cannot wrap `await` calls without blocking the event loop.

## Recommended Integration Approach

1. `uv add runtime-narrative`
2. Run async compatibility test above
3. If passes: wrap `handle_commit_trigger` + `handle_timer_trigger` in stories with stages matching existing pipeline steps — purely additive, no structural changes
4. Add `RuntimeNarrativeMiddleware` to webhook server
5. Expand to report generator + commit enhancer
6. Do NOT remove existing `logger.*` calls — runtime-narrative augments, doesn't replace

**Why:** `python_bridge.py` has the most opaque failure modes currently. Stage-level failure reporting would immediately identify whether breakage is in NLP, Ollama, Azure, or IPC.
