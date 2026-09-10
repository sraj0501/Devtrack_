---
name: DevTrack Sage implementation plan
description: Active planning for cross-harness capture, personal command knowledge, and later playback
type: project
---

The durable implementation plan is `docs/DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md`.

## Direction

DevTrack Sage is the local memory layer shared by coding-agent harnesses. Its scope extends
beyond Git. Existing `gitsage` package names and legacy `devtrack sage` commands matter
only as migration constraints.

DevTrack Sage is entirely Go-native. Port the required behavior and regression coverage from
`D:\git_apps\ai_sessions_skills`; production Sage must not spawn Python or require a Python
environment. The pinned functional reference for `tool/` is
`94a2544f8c85a630fa8b5d9a94d9938121aef11b`. Port for behavioral parity rather than copying
Python-specific process or file structure.

The reference implementation is the working tree at `D:\git_apps\ai_sessions_skills`.
Use it to inform the command/tool capture and self-writing knowledge pipeline. Treat LLM
output capture and session playback as designs to implement later, not proven reference
features.

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
devtrack sage install-hooks
devtrack sage playback start [title]
devtrack sage playback stop
devtrack sage playback status|list|show
```

A hidden, non-interactive hook endpoint accepts harness lifecycle and post-tool-use events.
Hooks must return quickly, print nothing, perform no model call, and never break an agent
session.

## Ownership and architecture

- Capture, playback, raw events, indexes, and rendered artifacts are Go-client-owned,
  SQLite/local-filesystem state. They do not require the Python server.
- Hooks write small immutable spool events. The DevTrack daemon imports and processes
  them asynchronously; do not port a detached Python watcher literally.
- SQLite is the canonical searchable index. Deterministically rendered Markdown is the
  portable human-readable knowledge/playback artifact.
- Extract reusable local/OpenAI-compatible model transport from `gitsage/llm.go` instead
  of creating a second provider implementation.
- Expose read-only knowledge and playback retrieval through DevTrack MCP after the local
  CLI is stable.
- A DevTrack work session may be linked as business context, but a harness recording has
  its own identity because agent threads can overlap one work session.
- Raw playback is local-only and must not enter `client_events`, PostgreSQL, telemetry, or
  a configured remote server by default.

## Implementation phases

### 1. Capture foundation

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

### 3. Command-only playback

- Implement `playback start/stop/status/list/show` with states `inactive`, `recording`,
  `processing`, `complete`, and `failed`.
- Bind a recording to the invoking harness thread and preserve exact commands, repeated
  actions, order, timestamps, cwd, exit status, material tool events, and concise evidence.
- Stopping persists the transition and returns immediately; finalization is asynchronous
  and crash-recoverable.
- Produce one durable Markdown artifact per finalized playback. Interrupted sessions
  remain recoverable and are never silently discarded.

### 4. Visible-reasoning playback

- Add harness adapters incrementally, starting with the best-supported visible transcript
  source. Internal Codex SQLite formats remain feature-flagged and failure-isolated.
- Record only visible primary-model summaries of intent, observation, decision, outcome,
  and verification. Never claim access to or reconstruct private chain-of-thought.
- Every rendered reasoning statement must be an exact substring of a source transcript
  event. Missing fields render as `Not recorded`.
- Harnesses without visible assistant-message access produce explicitly labelled
  command-only playback.

### 5. Integration and migration

- Add read-only MCP tools to search personal command knowledge and retrieve playbacks.
- Add a TUI browser only after CLI contracts are stable.
- Deprecate `sage ask`, `sage do`, and `sage interactive` for one compatibility release.
- Do not delete the entire `gitsage` package: retain or relocate commit enhancement, PR
  discovery, conflict helpers, `GitOps`, and shared LLM transport consumed elsewhere.

## Privacy and fidelity invariants

- Normal knowledge capture stores normalized commands without argument values where
  possible. Playback is manually activated because exact commands and output can contain
  secrets, customer data, and personal paths.
- Playback artifacts are private and gitignored by default; they are never automatically
  committed, pushed, synchronized, or submitted to a cloud model.
- Add retention and deletion controls before calling playback production-ready.
- Preserve original ordering and repetitions before knowledge normalization or dedupe.
- No Ollama, llama.cpp, cloud model, or Python call occurs on the hook hot path.

## Planning sequence

1. Build a port-parity matrix from every in-scope Python module and regression test at the pinned
   reference revision; assign a Go package, Go test, and milestone to each behavior.
2. Audit current `devtrack sage` commands and all consumers of the `gitsage` package.
3. Define the first milestone's versioned Go event/spool contract and sanitized fixture matrix.
4. Design migration and compatibility boundaries for legacy Sage commands.
5. Break capture foundation and searchable knowledge into board-ready implementation tasks.
6. Keep playback outside the first release milestone.

## First milestone boundary

The first milestone is complete when supported harnesses feed one local DevTrack knowledge
store; repeated actions deduplicate; new actions are asynchronously documented and
searchable; pause/status/log/doctor are truthful; hooks cannot block the agent; and no raw
capture leaves the machine. Playback is not part of this milestone.
