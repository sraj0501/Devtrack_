# Next Steps — DevTrack Sage

_Updated 2026-09-20. This file lists active work only. Completed delivery and validation history
lives in `Data/agent_logs/project_board.md`, release notes, and Git history._

## Current product initiative

Plan **DevTrack Sage** as DevTrack's local, cross-harness memory layer. The product is no longer
named Git Sage because its scope extends beyond Git operations to commands, tool activity,
and searchable personal knowledge across coding-agent harnesses.

The `devtrack sage` namespace is exclusively for the cross-harness command-knowledge product.
Implement the product behavior in the authoritative reference repository
`D:\git_apps\ai_sessions_skills` at commit `b85a1ab` as a Go-native DevTrack subsystem.

The active milestone is complete only when reliable local capture becomes self-writing command
knowledge: normalized signatures, deduplication, routing, local-model structured distillation,
deterministic topic Markdown, correction-stable routes, safe local commits, search, logs, lifecycle
control, and doctor diagnostics. Features absent from the reference are outside this milestone.

The executable milestones, architecture, acceptance criteria, and risk register are in
[DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md](DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md). Supporting active
context is maintained in `.claude/memory/project_sage_session_memory.md`. SAGE-001 established the
contract and port boundary as TASK-155. SAGE-002 is complete as TASK-156: the atomic bounded
spool, append-only SQLite event store, daemon importer, and opt-in read-only Codex history adapter
are implemented, hook installation/removal is reversible, and the isolated packaged capture
journey passes. Continue SAGE-003 with optional distillation and retry semantics now that the
model-free searchable index slice is implemented. SAGE-003 remains incomplete until it produces
the reference-compatible structured Markdown knowledge and passes the refreshed parity matrix.
Sanitized observed hook fixtures
remain an external harness-validation gate.
The selective harness registry and external plugin boundary are now fixed in
`SAGE_HARNESS_PLUGIN_CONTRACT.md`; new adapters must plug into that boundary rather than adding
global installer switches or harness-specific storage.
`Data/agent_logs/project_board.md` remains the task-ID authority.

## Open release follow-ups

- Qualify the admin review journey against packaged release artifacts.
- Capture and privacy-review the approved launch screenshots and video.
- Record the exact approved Glama listing path and add its score badge to
  `punkpeye/awesome-mcp-servers` PR #13608.

Clean Windows installation and full Managed Linux validation were confirmed complete by the owner
on 2026-09-10 and must not be reintroduced as pending work.

## Planning boundary

DevTrack Sage remains local-first and Go-client-owned. Hooks must be silent, bounded, model-free,
and failure-isolated. Raw harness payloads never enter PostgreSQL, telemetry, or remote
synchronization.

The implementation is a complete Go port of the required behavior and tests from the authoritative
`b85a1ab` Python
reference. Python is development evidence only: shipped Sage code must not invoke it or depend on
its environment. Port completion is tracked by behavioral parity, including edge cases, rather
than by translating files line-for-line.
