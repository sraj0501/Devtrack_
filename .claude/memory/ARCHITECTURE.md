# DevTrack Architecture & Patterns

## Two-Layer Architecture

```
User Git Activity / Cron Timer
        ↓
┌──────────────────┐     TCP IPC (JSON)     ┌──────────────────────────┐
│  Go Daemon       │ ──────────────────────▶ │  Python Bridge           │
│  devtrack-bin/   │                         │  python_bridge.py        │
│                  │ ◀────────────────────── │  - NLP parsing (spaCy)   │
│  - git_monitor   │    task_update / ack    │  - LLM enhancement       │
│  - scheduler     │                         │  - TUI user prompts      │
│  - ipc (server)  │                         │  - Report generation     │
│  - database      │                         │  - Project mgmt APIs     │
│  - cli           │                         │  - Learning (persistent) │
└──────────────────┘                         └──────────────────────────┘
        ↓                                              ↓
  SQLite (Data/)                        Azure/GitHub/Jira/Graph APIs
  PID/Logs (Data/)                      Microsoft Teams/Outlook
```

## Core Components

### Go Layer (devtrack-bin/)

**File Roles**:
- `main.go` - Entry point; routes CLI args or delegates `git` to shell wrapper
- `cli.go` - All CLI command implementations (start, stop, status, logs, enable-learning, etc.)
- `daemon.go` - Lifecycle management (PID file, signals, Python bridge process)
- `integrated.go` - IntegratedMonitor: wires git monitor, scheduler, IPC server
- `git_monitor.go` - fsnotify-based Git watcher; fires `commit_trigger` on commits
- `scheduler.go` - Cron-based periodic trigger; fires `timer_trigger`
- `ipc.go` - TCP IPC server (Go side); JSON-delimited messages
- `database.go` - SQLite via modernc.org/sqlite; trigger history & task updates
- `config.go` - YAML config struct; all runtime values via config_env.go
- `config_env.go` - **Single source of truth** for env var names
- `learning.go` - Personalized AI learning consent & profile management

### Python Layer (backend/ + python_bridge.py)

**Module Organization**:
- `python_bridge.py` - Entry point started by Go daemon; connects to IPC, dispatches triggers
- `backend/config.py` - Centralized config (all modules use this, not os.getenv)
- `backend/ipc_client.py` - TCP IPC client (Python side); mirrors Go message types
- `backend/nlp_parser.py` - spaCy NLP parser; extracts tasks from text (with git_context support)
- `backend/commit_message_enhancer.py` - Git context extraction & AI prompt enhancement
- `backend/conflict_auto_resolver.py` - Intelligent merge conflict resolution
- `backend/work_update_enhancer.py` - Git context enrichment for work updates
- `backend/description_enhancer.py` - Ollama-based enhancement & categorization
- `backend/user_prompt.py` - Terminal TUI for interactive prompts
- `backend/daily_report_generator.py` - AI-enhanced daily/weekly reports
- `backend/task_matcher.py` - Fuzzy + semantic matching to tracked tasks
- `backend/personalized_ai.py` - Talk Like You: learns & mimics user communication
- `backend/learning_integration.py` - Learning integration layer
- `backend/data_collectors.py` - Teams/Azure/Outlook data collection
- `backend/llm/` - Multi-provider LLM abstraction (provider_factory.py builds fallback chain)
- `backend/jira/`, `backend/github/`, `backend/azure/`, `backend/msgraph_python/` - External integrations

**Key Pattern**: All subsystems imported with try/except, individually gated; degrades gracefully

## IPC Message Protocol

**CRITICAL**: Message types defined in TWO PLACES - MUST STAY IN SYNC:
1. Go: `devtrack-bin/ipc.go` (MessageType constants)
2. Python: `backend/ipc_client.py` (MessageType enum)

**Format**: JSON-newline-delimited over TCP
**Default**: 127.0.0.1:35893 (configurable via IPC_HOST/IPC_PORT)

**Standard Flow**:
```
Go Daemon (server) → Sends trigger → Python Bridge (client)
Python Bridge → Processes → Sends task_update → Go Daemon
Go Daemon → Saves to DB → Sends ack → Python Bridge
```

## Configuration Pattern: NO Defaults Approach

**Single Source of Truth**: `.env` file (configurable via DEVTRACK_ENV_FILE)
- All required values must be explicitly set
- Missing env var → clear error message (no silent failures)
- Allows explicit configuration for different environments

Go reads via `config_env.go` functions:
```go
// All functions panic if env var missing with clear error:
GetIPCConnectTimeoutSecs() int  // → IPC_CONNECT_TIMEOUT_SECS (required)
GetProjectRoot() string         // → PROJECT_ROOT
GetWorkspaceDir() string        // → DEVTRACK_WORKSPACE
GetIPCHost() string             // → IPC_HOST
GetIPCPort() int                // → IPC_PORT
```

Python reads via `backend/config.py` functions:
```python
# All functions raise ConfigError if env var missing (no defaults)
from backend.config import (
    get, get_int, get_bool, get_path,
    get_http_timeout_short,      # → HTTP_TIMEOUT_SHORT (10 sec)
    get_http_timeout,            # → HTTP_TIMEOUT (30 sec)
    get_http_timeout_long,       # → HTTP_TIMEOUT_LONG (60 sec)
    get_ollama_host,             # → OLLAMA_HOST
    get_lmstudio_host,           # → LMSTUDIO_HOST
    get_git_sage_default_model,  # → GIT_SAGE_DEFAULT_MODEL
    get_ipc_retry_delay_ms,      # → IPC_RETRY_DELAY_MS
    get_prompt_timeout_simple,   # → PROMPT_TIMEOUT_SIMPLE_SECS
    get_prompt_timeout_work,     # → PROMPT_TIMEOUT_WORK_SECS
    get_prompt_timeout_task,     # → PROMPT_TIMEOUT_TASK_SECS
    get_llm_request_timeout_secs,     # → LLM_REQUEST_TIMEOUT_SECS
    get_sentiment_analysis_window_minutes,  # → SENTIMENT_ANALYSIS_WINDOW_MINUTES
)
```

**12 Required Configuration Variables**:
1. IPC_CONNECT_TIMEOUT_SECS - IPC connection timeout (seconds)
2. HTTP_TIMEOUT_SHORT - Short HTTP operations (seconds)
3. HTTP_TIMEOUT - Standard HTTP operations (seconds)
4. HTTP_TIMEOUT_LONG - Long HTTP operations (seconds)
5. IPC_RETRY_DELAY_MS - IPC retry delay (milliseconds)
6. OLLAMA_HOST - Ollama server URL
7. LMSTUDIO_HOST - LM Studio server URL
8. GIT_SAGE_DEFAULT_MODEL - Default LLM model
9. PROMPT_TIMEOUT_SIMPLE_SECS - Simple prompt timeout (seconds)
10. PROMPT_TIMEOUT_WORK_SECS - Work update prompt timeout (seconds)
11. PROMPT_TIMEOUT_TASK_SECS - Task prompt timeout (seconds)
12. LLM_REQUEST_TIMEOUT_SECS - LLM API request timeout (seconds)

**Plus Analysis Config**:
- SENTIMENT_ANALYSIS_WINDOW_MINUTES - Sentiment analysis time window (minutes)

**No hardcoded fallback values for ANY configuration** - everything must be in .env

## Git Integration Pattern (Phases 1-3)

**Phase 1: Commit Enhancer**
- `commit_message_enhancer.py`: `get_git_context()` extracts branch, PR, commits, diff stats
- Called from: `handle_commit_trigger()` in python_bridge.py
- Input: commit message (from Go)
- Output: Enhanced commit message with git context

**Phase 2A: Conflict Resolver**
- `conflict_auto_resolver.py`: Detects and resolves merge conflicts
- Called from: `_check_and_resolve_conflicts()` in python_bridge.py
- Input: repo path (from .env)
- Output: {status: success|partial|failed, resolved: [...], unresolvable: [...]}

**Phase 2B: Work Update Enhancer**
- `work_update_enhancer.py`: Injects git context before NLP parsing
- `nlp_parser.py`: Enhanced with `repo_path` parameter, auto-detects PR/issue from git context
- Called from: `handle_timer_trigger()` in python_bridge.py
- Input: user's work update text
- Output: Enhanced text with git context injected

**Phase 3: Integration**
- All above integrated into python_bridge.py event pipeline
- All imports graceful fallback (if git-sage unavailable, continue with degraded functionality)
- All features optional, non-blocking

**Pattern**: Always pass `repo_path` through to parsers for context enrichment

## LLM Provider Pattern

**Multi-Provider Fallback Chain**:
```python
# provider_factory.py builds chain based on available credentials:
providers = []
if openai_key:   providers.append(OpenAIProvider)
if claude_key:   providers.append(AnthropicProvider)
providers.append(OllamaProvider)  # Always last (fallback)

# Usage tries each in order until one succeeds
```

**Graceful Degradation**: If all providers fail, system continues with heuristics

**Models Used**:
- Ollama: mistral (or orca-mini for lower hardware)
- OpenAI: gpt-4, gpt-4-turbo, gpt-3.5-turbo
- Anthropic: claude-3-opus, claude-3-sonnet

## Testing Patterns

**Test Structure**:
- `backend/tests/conftest.py` - Fixtures & test setup
- All tests use `uv run pytest`
- Tests can reset LLM provider cache to avoid cross-test contamination

**Test Commands**:
```bash
uv run pytest backend/tests/                    # All tests
uv run pytest backend/tests/test_file.py        # Single file
uv run pytest backend/tests/test_file.py::TestClass::test_method  # Single test
uv run pytest --cov=backend                     # With coverage
```

## Code Quality Standards

1. **Type Hints**: 100% on public methods
   ```python
   def method(self, param: str, count: int = 10) -> Optional[Dict[str, Any]]:
   ```

2. **Docstrings**: Module, class, and method level
   ```python
   """One-line summary.

   Longer description if needed.

   Args:
       param: Description

   Returns:
       Description of return value
   """
   ```

3. **Error Handling**: Try/except with logging
   ```python
   try:
       # operation
   except SpecificError as e:
       logger.error(f"Operation failed: {e}")
       # graceful fallback
   ```

4. **Enum-Based Constants**: Never magic strings
   ```python
   class Status(Enum):
       ACTIVE = "active"
       CLOSED = "closed"
   ```

5. **Optional Imports**: Graceful degradation
   ```python
   try:
       from optional_module import Feature
       HAS_FEATURE = True
   except ImportError:
       HAS_FEATURE = False

   # Later:
   if HAS_FEATURE:
       # use feature
   else:
       # fallback
   ```

## Key Design Decisions

1. **Two Languages**: Go for system-level (monitoring, IPC, DB), Python for AI/NLP
   - Pro: Best tool for each job, leverage ecosystem
   - Con: Two language stack to maintain

2. **IPC over Direct**: Separate processes with TCP communication
   - Pro: Isolation, Python can restart without Go daemon dying
   - Con: Slight latency overhead (negligible)

3. **Offline-First**: Ollama local by default, cloud optional
   - Pro: Privacy, speed, no vendor lock-in
   - Con: Requires Ollama installation

4. **Graceful Degradation**: All features optional
   - Pro: System works even if LLM, AI, or database unavailable
   - Con: More code paths to test

5. **Single .env**: All config from one place
   - Pro: Simple, predictable
   - Con: Long .env file with 150+ variables

## Common Development Tasks

### Adding New CLI Command
1. Create handler in `devtrack-bin/cli.go`
2. Wire into CLI switch statement
3. Add help text
4. Test with `devtrack <command>`

### Adding New Trigger Type
1. Add MessageType constant to `devtrack-bin/ipc.go`
2. Add to Python enum in `backend/ipc_client.py` (KEEP IN SYNC!)
3. Add handler in `python_bridge.py`
4. Wire scheduler or git_monitor to fire trigger

### Adding New Python Module
1. Create in `backend/` directory
2. Import with try/except at top of files using it
3. Set availability flag (e.g., `HAS_MODULE = True/False`)
4. Use availability flag to gate features
5. Add tests in `backend/tests/`
6. Document in appropriate docs/

### Integrating External API
1. Create client in `backend/{service}/client.py`
2. Use existing patterns (see `backend/azure/`, `backend/github/`)
3. Wrap in try/except
4. Log operations
5. Handle errors gracefully
6. Add to config.py for credentials
7. Add tests

## Debugging Patterns

**Enable Debug Logging**:
```bash
LOG_LEVEL=debug devtrack start
```

**Check IPC Communication**:
```bash
# Terminal 1: Watch daemon logs
devtrack logs -f

# Terminal 2: Trigger action
devtrack force-trigger
# Watch logs in terminal 1
```

**Test Python Bridge Directly**:
```python
from backend.nlp_parser import parse_update
result = parse_update("PR #42 (2h)")
print(result)
```

**Inspect Database**:
```bash
sqlite3 Data/db/devtrack.db
sqlite> SELECT * FROM triggers ORDER BY trigger_time DESC LIMIT 10;
```

## File Organization Principles

- **Python**: One module per file, directories for related modules
- **Go**: Related functions in same file, separate files by concern
- **Config**: All in `.env`, accessed via config functions
- **Tests**: Mirror source structure in `backend/tests/`
- **Docs**: User-facing in `docs/`, reference at root

## Performance Characteristics

- **Git monitoring**: Sub-second (fsnotify)
- **NLP parsing**: ~100-500ms (depends on text length)
- **LLM enhancement**: ~1-30s (Ollama) or ~2-10s (cloud APIs)
- **Database writes**: <5ms
- **IPC roundtrip**: <50ms
- **Total work update flow**: ~5-40s depending on AI provider

---

**Last Updated**: March 11, 2026
**Current Phase**: 4 (Project Management) + Personalization integration
**Architecture Stability**: High (well-established patterns)
