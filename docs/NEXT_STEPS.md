# Next Steps — DevTrack Sage

_Updated 2026-09-10. This file lists active work only. Completed delivery and validation history
lives in `Data/agent_logs/project_board.md`, release notes, and Git history._

## Current product initiative

Plan **DevTrack Sage** as DevTrack's local, cross-harness memory layer. The product is no longer
named Git Sage because its scope extends beyond Git operations to commands, tool activity,
searchable personal knowledge, and later opt-in session playback across coding-agent harnesses.

The existing `devtrack sage ask/do/pr/interactive` behavior and the Go `gitsage` package are shipped
legacy surfaces. Preserve commit enhancement, PR discovery, conflict handling, Git operations, and
shared model transport while the migration plan defines compatible command and package boundaries.

The first implementation slice should stop at reliable local capture plus searchable command
knowledge. Playback and visible-reasoning enrichment are later milestones and must not be folded
into the capture foundation.

The executable milestones, architecture, acceptance criteria, and risk register are in
[DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md](DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md). Supporting active
context is maintained in `.claude/memory/project_sage_session_memory.md`. No DevTrack Sage
implementation task has been approved or assigned yet; `Data/agent_logs/project_board.md` remains
the task-ID authority.

## Open release follow-ups

- Qualify the admin review journey against packaged release artifacts.
- Capture and privacy-review the approved launch screenshots and video.
- Record the exact approved Glama listing path and add its score badge to
  `punkpeye/awesome-mcp-servers` PR #13608.

Clean Windows installation and full Managed Linux validation were confirmed complete by the owner
on 2026-09-10 and must not be reintroduced as pending work.

## Planning boundary

DevTrack Sage remains local-first and Go-client-owned. Hooks must be silent, bounded, model-free,
and failure-isolated. Raw capture and playback data stay local by default and never enter
PostgreSQL, telemetry, or remote synchronization implicitly.
