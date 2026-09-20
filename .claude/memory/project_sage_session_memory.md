---
name: DevTrack Sage implementation plan
description: Active planning for cross-harness capture and self-writing personal command knowledge
type: project
---

The durable implementation plan is `docs/DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md`.

## Direction

DevTrack Sage is the local memory layer shared by coding-agent harnesses. Its scope extends
beyond Git. The `devtrack sage` namespace belongs exclusively to the cross-harness command
knowledge product.

DevTrack Sage is entirely Go-native. Port the required behavior and regression coverage from
`D:\git_apps\ai_sessions_skills`; production Sage must not spawn Python or require a Python
environment. The authoritative reference is current commit `b85a1ab`. Port for behavioral parity
rather than copying Python-specific process boundaries.

The reference implementation is the working tree at `D:\git_apps\ai_sessions_skills`.
It defines command/tool capture, signatures, deduplication, routing, local-model distillation,
structured entries, deterministic Markdown knowledge files, search, correction-stable routes,
safe local commits, logging, lifecycle control, diagnostics, and supported harness behavior.
Features absent from that reference are outside the parity milestone.

This direction serves DevTrack's durable positioning: harness-side generation is
commoditized, while an ambient private memory layer underneath Claude Code, Codex,
Copilot, Cursor, Devin, and OpenCode becomes more valuable as the harness ecosystem grows.

## Product surface

```text
devtrack sage status
devtrack sage pause|resume
devtrack sage search <query>
devtrack sage topics
devtrack sage doctor
devtrack sage log
devtrack sage routes
devtrack sage route <binary> <topic>
devtrack sage merge <source> <destination>
devtrack sage harness list|install|uninstall
```

A hidden, non-interactive hook endpoint accepts harness lifecycle and post-tool-use events.
Hooks must return quickly, print nothing, perform no model call, and never break an agent
session.

## Ownership and architecture

- Capture, normalized events, indexes, and rendered artifacts are Go-client-owned,
  SQLite/local-filesystem state. They do not require the Python server.
- Hooks write small immutable spool events. The DevTrack daemon imports and processes
  them asynchronously; do not port a detached Python watcher literally.
- SQLite owns queues, processing state, and the searchable index. Deterministically rendered
  Markdown topic files and their index are the durable, user-visible knowledge artifacts.
- Implement reusable local/OpenAI-compatible transport in `internal/llmclient`; Sage must not
  depend on an unrelated repository-assistant package.
- Expose read-only knowledge retrieval through DevTrack MCP after the local
  CLI is stable.
- A DevTrack work session may be linked as business context, but a harness recording has
  its own identity because agent threads can overlap one work session.
- Raw harness payloads must not enter `client_events`, PostgreSQL, telemetry, or a configured
  remote server.

## Implementation phases

### 1. Capture foundation

Current state (2026-09-20): SAGE-001 and SAGE-002 are implemented. Capture has the atomic spool, bounded
importer/quarantine, append-only SQLite event store, daemon ownership, and feature-flagged read-only
Codex `vscode`/`cli` history poller. Idempotent install/remove preserves unrelated configuration;
Windows selects history mode for terminal-host compatibility. The packaged capture journey passes.
SAGE-003 has an initial searchable SQLite grouping/index slice; it is not complete until it writes
the reference-compatible structured Markdown knowledge, routes topics, retries model failures, and
passes the refreshed parity matrix.

- Define versioned harness-event and spool schemas.
- Port payload duck-typing for shell, search, edit, success/failure, cwd, project, and
  harness session identifiers.
- Port signature normalization, deduplication, bounded queues, activity decisions,
  pause/resume, status, logs, and doctor diagnostics.
- Implement idempotent hook installation/removal without overwriting unrelated harness
  configuration.
- Port cross-platform payload fixtures and concurrency/failure tests before expanding
  behavior.

### 2. Personal command knowledge

- Add daemon-side asynchronous batching and local-model distillation.
- Keep routing, dedupe, file mutation, and Markdown formatting deterministic in Go; the
  model returns validated structured fields and never directly edits the knowledge base.
- Add topic routing, correction-stable routes, merge/refile operations, skipped-action
  reasons, search, and Markdown export.
- Default to local Ollama and degrade safely when no model is available. Captures remain
  queued for retry; model failure is never a verdict that an event is useless.

### 3. Cross-harness parity

- Port adapters and fixtures for Devin CLI, Claude Code, Codex CLI, Copilot CLI, OpenCode,
  Cursor, and the supported IDE-history path.
- Apply one silence, latency, privacy, deduplication, install, removal, and failure-isolation
  contract to every advertised harness.
- Preserve unrelated harness configuration during install, reinstall, and uninstall.

### 4. Knowledge operations and diagnostics

- Port activity logs, decision states, truthful doctor output, route inspection/correction,
  merge/refile, skipped-action records, and deterministic topic-index maintenance.
- Port safe local knowledge commits without touching unrelated staged or unstaged work and
  without ever pushing.
- Add bounded retry state so model outages never become permanent skip decisions.

### 5. Read-only integration

- Add read-only MCP tools to search personal command knowledge.
- Add a TUI browser only after CLI contracts are stable.

## Privacy and fidelity invariants

- Normal knowledge capture stores normalized commands without private argument values where
  possible. Raw command output, user messages, and reasoning are not part of normal capture.
- Runtime queues and logs are private and gitignored. Knowledge files may be committed locally
  only through the reference-compatible safe commit path; Sage never pushes them.
- No Ollama, llama.cpp, cloud model, or Python call occurs on the hook hot path.

## Planning sequence

1. Build a port-parity matrix from every in-scope Python module and regression test at the pinned
   reference revision; assign a Go package, Go test, and milestone to each behavior.
2. Define the versioned Go event/spool contract and sanitized fixture matrix.
3. Port searchable, self-writing Markdown knowledge behavior before adding unrelated features.
4. Add harnesses one at a time against the shared adapter acceptance suite.
5. Treat the parity matrix as the completion authority; file presence is not completion.

## First milestone boundary

The first milestone is complete when supported harnesses feed one local DevTrack knowledge
store; repeated actions deduplicate; new actions are asynchronously documented and
searchable; pause/status/log/doctor are truthful; hooks cannot block the agent; and no raw
capture leaves the machine. The milestone also requires deterministic topic Markdown, routing,
structured what/why/example/notes entries, and safe retry when the local model is unavailable.
