# DevTrack Sage and the retired Git Sage name

DevTrack Sage is the Go-native, local command-knowledge subsystem bundled with `devtrack`. It
captures privacy-minimized command activity from supported coding harnesses, imports it into local
SQLite, and makes repeated command knowledge searchable. It is not a repository chat agent and it
does not execute Git operations.

Older documentation used **Git Sage** for a different repository-agent experiment. That surface is
retired: `sage ask`, `sage do`, `sage pr`, `sage interactive`, `sage git`, and free-form questions
no longer exist. Do not use `GIT_SAGE_*` settings for current Sage; a few Python-server reads remain
only as legacy model fallbacks. The independently useful enhanced-commit workflow remains available
as `devtrack git`, while `devtrack sage` now means command capture and personal knowledge.

| Need | Current command |
|---|---|
| Commit-message assistance or Git pass-through | `devtrack git ...` |
| Silent command capture and local knowledge search | `devtrack sage ...` |
| Repository questions or autonomous Git actions | Not provided |

## Install capture

```bash
devtrack sage harness list
devtrack sage harness install codex
devtrack sage harness install claude
devtrack sage harness install copilot
devtrack sage harness install opencode
devtrack sage harness install cursor
devtrack sage harness install devin
devtrack sage status
```

Installed hooks are silent, bounded, model-free, and fail-open. They omit raw command output,
reasoning, messages, and working-directory paths. On Windows, Codex uses an opt-in read-only history
adapter to avoid console-host compatibility problems. Claude Code uses owned `PostToolUse` hooks
for Bash/search/edit tools and a Bash-only `PostToolUseFailure` hook while preserving unrelated
`~/.claude/settings.json` entries. GitHub Copilot CLI uses one owned version-1 `postToolUse` entry
with portable Bash and PowerShell commands while preserving foreign entries in
`~/.copilot/hooks/ai-session-skills.json`. OpenCode uses one owned user plugin that observes both
`tool.execute.before` and `tool.execute.after` without adding session lifecycle work.
Cursor and Devin CLI are independently managed adapters; install only the harnesses you use.

Pause or remove capture without changing unrelated harness configuration:

```bash
devtrack sage pause
devtrack sage resume
devtrack sage harness uninstall codex
devtrack sage harness uninstall claude
devtrack sage harness uninstall copilot
devtrack sage harness uninstall opencode
devtrack sage harness uninstall cursor
devtrack sage harness uninstall devin
```

## Search local knowledge

```bash
devtrack sage search "rebase"
devtrack sage search "status command" --topic git
devtrack sage topics
devtrack sage doctor
```

Stable v3.2.0 includes durable SQLite claims and retries, daemon-owned background distillation,
deterministic Markdown topic files, correction-stable routing, merge/refile, local knowledge commits,
and read-only Sage MCP tools. Capture does not wait for a model. Search reconciles its local index
with the current Markdown snapshot.

Bounded background-worker settings include:

```bash
DEVTRACK_SAGE_MODEL_TIMEOUT_SECS=180  # allowed: 5–1800 seconds
DEVTRACK_SAGE_IDLE_POLL_MS=500        # allowed: 50–5000 ms
DEVTRACK_SAGE_RETRY_DELAY_SECS=2      # allowed: 1–60 seconds
```

Model failures remain retryable without blocking command capture.

## Git workflows

Use normal Git commands for repository operations. `devtrack git commit` remains an explicit,
separate enhanced-commit workflow and uses the shared `LLM_PROVIDER`, `OLLAMA_HOST`, and
`OLLAMA_MODEL` settings. Removed `sage ask`, `sage do`, `sage pr`, `sage interactive`, `sage git`,
and free-form Sage questions fail clearly instead of starting a model or Git operation.
