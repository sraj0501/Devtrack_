---
name: DevTrack Sage implementation plan
description: Active planning for cross-harness capture and self-writing personal command knowledge
type: project
---

The durable implementation plan is `docs/DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md`.

## Direction

DevTrack Sage is the local memory layer shared by coding-agent harnesses. The `devtrack sage`
namespace belongs exclusively to the cross-harness, self-writing command-knowledge product.

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
- Implement reusable local/OpenAI-compatible transport in `internal/llmclient`.
- Expose read-only knowledge retrieval through DevTrack MCP after the local
  CLI is stable.
- A DevTrack work session may be linked as business context, but a harness recording has
  its own identity because agent threads can overlap one work session.
- Raw harness payloads must not enter `client_events`, PostgreSQL, telemetry, or a configured
  remote server.

## Active implementation boundary

The capture foundation and initial searchable SQLite grouping/index exist. They are supporting
infrastructure, not completion of SAGE-003. SAGE-003 remains active until all of the following are
implemented and covered by the refreshed reference-parity matrix:

- asynchronous local-model distillation into validated `what`, `why`, `example`, and `notes`
  fields;
- deterministic topic Markdown files and topic-index maintenance;
- deterministic routing, correction-stable routes, and merge/refile operations;
- safe local knowledge commits that never disturb unrelated staged or unstaged work and never
  push;
- bounded retries for model and processing failures, with no permanent skip caused by model
  unavailability;
- truthful status, activity logs, skipped-action reasons, and doctor diagnostics; and
- all applicable regression scenarios from the authoritative reference.

Keep routing, deduplication, file mutation, and Markdown rendering deterministic in Go. The model
returns structured fields and never edits the knowledge base. Default to local Ollama and retain
captures for retry when no model is available.

After SAGE-003, complete adapter parity for Devin CLI, Claude Code, Codex CLI, Copilot CLI,
OpenCode, Cursor, and the supported IDE-history path. Every advertised harness must share the same
silence, latency, privacy, deduplication, install/removal, and failure-isolation contract. Read-only
MCP knowledge retrieval follows only after the local CLI contract is stable.

## Privacy and fidelity invariants

- Normal knowledge capture stores normalized commands without private argument values where
  possible. Raw command output, user messages, and reasoning are not part of normal capture.
- Runtime queues and logs are private and gitignored. Knowledge files may be committed locally
  only through the reference-compatible safe commit path; Sage never pushes them.
- No Ollama, llama.cpp, cloud model, or Python call occurs on the hook hot path.

## Active execution sequence

1. Keep the port-parity matrix mapped to every in-scope module and regression test at `b85a1ab`.
2. Finish the active SAGE-003 boundary before adding unrelated features.
3. Add harnesses one at a time against the shared adapter acceptance suite.
4. Treat parity behavior and tests as the completion authority; file presence is not completion.

## First milestone boundary

The first milestone is complete when supported harnesses feed one local DevTrack knowledge
store; repeated actions deduplicate; new actions are asynchronously documented and
searchable; pause/status/log/doctor are truthful; hooks cannot block the agent; and no raw
capture leaves the machine. The milestone also requires deterministic topic Markdown, routing,
structured what/why/example/notes entries, and safe retry when the local model is unavailable.
