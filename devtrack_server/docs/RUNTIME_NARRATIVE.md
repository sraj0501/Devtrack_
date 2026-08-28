# runtime-narrative in DevTrack

> Living document — updated alongside each implementation phase.
> Think of this as notes taken while integrating the library, not a retrospective.

---

## What it is

[runtime-narrative](https://pypi.org/project/runtime-narrative/) (`>=0.1.0`) is a Python
observability library that wraps units of work in **stories** (a named request or operation)
made up of **stages** (named sub-steps). It emits structured events — `StoryStarted`,
`StageStarted`, `StageCompleted`, `FailureOccurred`, `StoryCompleted` — to one or more
**renderers** (console or JSON file).

In DevTrack it serves two roles:

1. **Per-request timeline** — every HTTP request to the webhook server is automatically a
   story (via `RuntimeNarrativeMiddleware`). Stages inside the handler give per-step timing.

2. **Structured failure reporting** — when a critical stage raises an uncaught exception,
   `FailureOccurred` fires with exact file/line/function/source-line, the full stage
   timeline, and (when Ollama is running) an LLM-generated diagnosis from
   `OllamaFailureAnalyzer`.

---

## How it's integrated

### Middleware (automatic story per request)

```python
# backend/webhook_server.py
from runtime_narrative import RuntimeNarrativeMiddleware, JsonRenderer, OllamaFailureAnalyzer

app.add_middleware(
    RuntimeNarrativeMiddleware,
    renderers=[JsonRenderer(output=open("Data/logs/narrative.log", "a", encoding="utf-8"))],
    failure_analyzer=OllamaFailureAnalyzer(model="llama3.2", endpoint=f"{OLLAMA_HOST}/api/generate"),
)
```

Every request becomes a story named `"<METHOD> <path>"` (e.g. `"POST /trigger/commit"`).
The story context is set in a `contextvars.ContextVar` and is accessible from any code
running in the same async task — including code dispatched via `asyncio.to_thread`.

### Stage instrumentation (inside handlers)

```python
from runtime_narrative import stage as _stage   # imported at module level with fallback

with _stage("LLM task parse"):
    task_data = llm_task_parser.parse(commit_msg)

with _stage("PM sync"):
    workspace_router.route(...)
```

`_stage` is a context manager. On entry it emits `StageStarted`; on exit it emits
`StageCompleted` with `duration_seconds`. If an exception propagates *out* of the `with`
block, the story's `__exit__` emits `FailureOccurred` (see critical vs graceful below).

### Graceful fallback

Both the middleware and `_stage` are wrapped in `try/except (ImportError, TypeError)` so
the server runs normally if `runtime-narrative` is uninstalled.

---

## The log file: `narrative.log`

Location: `LOG_DIR/narrative.log` (override with `NARRATIVE_LOG_PATH`).
Format: one JSON object per line, UTF-8, append mode.

### Event types

#### `StoryStarted`
```json
{
  "event": "StoryStarted",
  "story_id": "838d3c54-015d-4c1f-9fd5-c3403898042c",
  "story_name": "POST /trigger/commit",
  "timestamp": "2026-06-02T01:13:07.952761"
}
```

#### `StageStarted` / `StageCompleted`
```json
{ "event": "StageStarted",   "story_id": "838d...", "stage_name": "LLM task parse",  "timestamp": "..." }
{ "event": "StageCompleted", "story_id": "838d...", "stage_name": "LLM task parse",  "duration_seconds": 0.042, "timestamp": "..." }
```

#### `StoryCompleted`
```json
{
  "event": "StoryCompleted",
  "story_id": "838d3c54-015d-4c1f-9fd5-c3403898042c",
  "story_name": "POST /trigger/commit",
  "success": true,
  "progress": { "percent": 100, "completed_stages": 3, "total_stages": 3 },
  "timestamp": "2026-06-02T01:13:07.962427"
}
```

#### `FailureOccurred` (only when a critical-path stage raises)
```json
{
  "event": "FailureOccurred",
  "story_id": "...",
  "story_name": "POST /trigger/ticket_sync",
  "stage_name": "Upsert 42 github tickets",
  "error_type": "OperationalError",
  "error_message": "connection refused",
  "location": { "filename": "ticket_db.py", "lineno": 125, "function": "upsert_ticket", "source_line": "conn.execute(stmt)" },
  "exception_chain": [...],
  "exact_cause": "PostgreSQL connection refused while writing the ticket store",
  "llm_analysis": "The PostgreSQL service configured by POSTGRES_URL is unreachable. Confirm the service is running, the host and port are reachable, and the database exists.",
  "stage_timeline": "Link work session (0.4ms) → LLM task parse (0.2ms) → [FAILED] Upsert 42 github tickets",
  "progress": { "percent": 66, "completed_stages": 2, "total_stages": 3 },
  "timestamp": "..."
}
```

---

## Correlating `narrative.log` with server logs

Every `logger.*` call during a request includes the story UUID in square brackets:

```
2026-06-02 01:13:07,953 - devtrack.webhook_server - INFO - [838d3c54-...] [HTTP commit] abc123 — feat: JWT refresh
```

Startup and background logs show `[-]` (no active story).

The `story_id` is also returned in HTTP responses:

```json
{ "status": "ok", "actions": [], "commit_hash": "abc123", "narrative_id": "838d3c54-..." }
```

The Go client can store `narrative_id` in the SQLite trigger record to link Go's trigger
log → Python's narrative.log.

---

## Adding stages to new code

### Two patterns: critical vs graceful

**Critical-path** — failure should fire `FailureOccurred` and return a non-2xx response.
The exception must propagate *out* of the `with _stage()` block:

```python
# try/except is OUTSIDE the stage — exception escapes into the story
with _stage("Write to Azure DevOps"):
    azure_client.add_comment(work_item_id, comment)   # raises on 401/timeout
```

If `azure_client.add_comment` raises, `FailureOccurred` fires with the exact line,
`OllamaFailureAnalyzer` diagnoses it, and FastAPI returns 500. The Go client handles
non-2xx from push calls gracefully.

**Graceful-degradation** — failure is expected, non-fatal, already logged. The exception
is swallowed *inside* the stage, so `FailureOccurred` never fires (correct):

```python
# try/except is INSIDE the stage — exception is swallowed, story continues
with _stage("Send Telegram reminder"):
    try:
        telegram.send(...)
    except Exception:
        logger.debug("Telegram unavailable (non-fatal)")
```

**Rule of thumb:** if the stage failing should abort the request → critical. If it's a
best-effort notification or optional enrichment → graceful.

### Stage naming conventions

- Use verb + noun: `"LLM task parse"`, `"PM sync"`, `"Force clear cache"`
- Include dynamic context when it adds value: `"Upsert 42 github tickets"`,
  `"LLM call [ollama/llama3.2]"`, `"Persona: Architect"`
- Keep names short enough to be readable in a one-line stage timeline

---

## OllamaFailureAnalyzer

Wired automatically at startup when `OLLAMA_HOST` is set and reachable (2-second ping
to `/api/tags`). Uses `GIT_SAGE_DEFAULT_MODEL` (same model as git-sage) for consistency.

```python
OllamaFailureAnalyzer(
    model="llama3.2",
    endpoint="http://localhost:11434/api/generate",
    timeout_seconds=12.0,   # default; increase for slow hardware
)
```

**How it works:** When `FailureOccurred` fires, the analyzer builds a prompt containing:
story name, failed stage name, error type + message, source line, full stage timeline,
and up to 30 lines of traceback. It sends this to Ollama via `/api/generate` with
`temperature=0` and writes the response into `llm_analysis` in the `FailureOccurred` event.

**When it doesn't fire:** If the try/except is inside the `with _stage()` block (graceful
pattern), the exception never propagates and the analyzer is never called. Only critical-
path stages trigger it.

**Timeout:** 12 seconds. If Ollama doesn't respond in time, `llm_analysis` is `null` in
the event — the rest of the `FailureOccurred` event is still emitted.

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `NARRATIVE_LOG_PATH` | `$LOG_DIR/narrative.log` | Override log file location |
| `NARRATIVE_RENDERER` | _(unset)_ | Set to `console` to add `ConsoleRenderer` alongside JSON (dev mode; requires `PYTHONIOENCODING=utf-8` on Windows) |
| `OLLAMA_HOST` | _(unset)_ | If set and reachable, wires `OllamaFailureAnalyzer` automatically |
| `GIT_SAGE_DEFAULT_MODEL` | `llama3.2` | Model used by `OllamaFailureAnalyzer` |
| `PYTHONIOENCODING` | _(system default)_ | **Must be `utf-8` on Windows** when `NARRATIVE_RENDERER=console` — `ConsoleRenderer` uses Unicode glyphs that crash `cp1252` |

---

## Known library gaps

These are issues in `runtime-narrative` itself (not DevTrack):

- **Progress always 0% mid-request** — `progress_percent` in `StoryCompleted` is correct
  (100%), but during a request it's always 0 because the library has no API to declare
  total stages upfront. `StoryCompleted` infers `total_stages` from `completed_stages` at
  the end.

- **No built-in log rotation** — `JsonRenderer` writes to a file indefinitely.
  Use an external log rotator (e.g. `logrotate` on Linux) or wrap the file handle with
  Python's `RotatingFileHandler`-style logic.

- **`ConsoleRenderer` crashes on Windows `cp1252`** — uses `▶` (U+25B6) which is not in
  Windows-1252. Fixed in DevTrack by defaulting to `JsonRenderer`. If you add
  `NARRATIVE_RENDERER=console`, set `PYTHONIOENCODING=utf-8` first.

---

## What's instrumented (Phases 1–3)

### Trigger handlers (`webhook_server.py`)

| Endpoint | Stages | Pattern |
|---|---|---|
| `POST /trigger/commit` | Link work session · LLM task parse · PM sync | Graceful |
| `POST /trigger/timer` | Check vacation mode · Check active session · Send Telegram reminder · Send Slack reminder | Graceful |
| `POST /trigger/ticket_sync` | Force clear cache _(force=true only)_ · Upsert N source tickets | **Critical** |

### LLM provider chain (`llm/provider_factory.py`)

| Stage | When |
|---|---|
| `LLM [ollama/llama3.2]` | Primary provider attempt using the configured/default local model |
| `LLM fallback [openai/gpt-4o-mini]` | First fallback (if primary returns None) |
| `LLM fallback [ollama/llama3.2]` | Final free fallback |

Providers swallow their own exceptions internally and return `None` on failure, so
`FailureOccurred` does not fire from the LLM chain itself — but stage timing shows
exactly how long each provider took before giving up.

### PM API (`workspace_router.py`)

| Stage | When |
|---|---|
| `Azure: fetch work items` | Before matching any commit |
| `Azure: comment on AB#N` | After a match is found |
| `Azure: create work item` | When `AZURE_SYNC_CREATE_ON_NO_MATCH=true` |
| `GitLab: fetch issues` | Before matching |
| `GitLab: comment on #N` | After match |
| `GitLab: create issue` | When create-on-no-match enabled |
| `GitHub: fetch issues` | Before matching |
| `GitHub: comment on #N` | After match |
| `GitHub: create issue` | When create-on-no-match enabled |

All PM API stages are **graceful** — try/except is inside the stage so PM failures
don't crash the commit trigger story.

### Inbound webhooks (`webhook_handlers.py`)

| Stage | When |
|---|---|
| `Azure: workitem.updated` | Azure DevOps service hook received |
| `Azure: workitem.commented` | Comment event |
| `Azure: workitem.created` | New work item event |
| `GitLab: Issue Hook` | GitLab issue event |
| `GitLab: Merge Request Hook` | MR event |
| `GitLab: Note Hook` | Comment event |

### Boardroom (`boardroom/session.py`)

| Stage | When |
|---|---|
| `Persona: Architect` | LLM call for Architect persona |
| `Persona: Security` | … |
| `Persona: PM` | … (7 personas total, run in parallel) |
| `Moderator synthesis` | Final SWOT + verdict LLM call |

### Report generation (`daily_report_generator.py`)

| Stage | When |
|---|---|
| `Gather report data` | `email_reporter.generate_daily_report()` call |
| `LLM insights` | Ollama AI enhancement (only when `include_ai=True`) |
| `Email delivery → recipient@domain` | `send_via_email()` call |

---

## Consuming narrative.log

### HTTP endpoints (API-key protected)

```
GET /narrative/recent?n=20     → { "stories": [...] }
GET /narrative/last-failure    → { "event": "FailureOccurred", ... } or {}
```

Both require the `X-DevTrack-API-Key` header (same key as trigger endpoints). The Go
client calls these via `trigger.HTTPTriggerClient.GetNarrativeRecent()` and
`GetNarrativeLastFailure()`.

### Go CLI

```bash
devtrack narrative          # last 20 stories, per-stage timing
devtrack narrative -n 50    # last 50 stories
```

`devtrack status` also surfaces the most recent `FailureOccurred` inline:
```
AI server:     connected  (http://localhost:8089)
Last failure:  01:26:32  "Upsert 2 github tickets"  POST /trigger/ticket_sync
               RuntimeError: Failed to create PostgreSQL engine: No module named 'psycopg2'
```

### Admin UI

The dashboard (`/admin/`) includes a **Request Narrative** card showing the last 20
stories in a table — endpoint, stage pills with timing, total duration, pass/fail badge,
and an expanded failure row with error details and LLM analysis when present. The table
auto-refreshes every 30 seconds via HTMX.

Stage pills are highlighted amber when duration ≥ 500ms (slow path indicator).

Direct HTMX partial: `GET /admin/_partials/narrative` (requires admin session cookie).
