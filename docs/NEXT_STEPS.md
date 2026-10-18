# Next Steps — DevTrack

_Updated 2026-10-18. This file lists active work only. Completed delivery and validation history
lives in `Data/agent_logs/project_board.md`, release notes, and Git history._

## Immediate repository sequence

TASK-169 is fully green on PR #291 at final head `bf963dd`, but remains unmerged. TASK-170 is active
on `features/TASK-170-sage-operational-closure`, stacked from that head. Its local retention,
quarantine, exact deletion, deterministic repair/cache rebuild, and Markdown export/import gates
pass on Linux and native Windows. Complete two documentation reconciliation passes, commit and push
through DevTrack, open the dedicated PR to `dev`, and wait for all final-head checks. Do not merge
either PR without explicit authorization.

The authoritative clean behavioral reference is
`C:\Work\git_apps\ai_sessions_skills` at intentional bug-fix HEAD `ea2e067`. The parity inventory
accounts for all 136 reference scenarios: 119 implemented, 17 adapted, 0 partial, and 0 pending.
Python remains development evidence only; shipped Sage is Go-native and local.

## Product boundary

`devtrack sage` is the local cross-harness command-knowledge product, not a repository-chat or
autonomous Git agent. Hooks remain silent, bounded, model-free, and fail-open. SQLite owns private
queues and derived retrieval state; deterministic Markdown is the portable knowledge authority.
Raw harness payloads never enter PostgreSQL, telemetry, MCP, or remote synchronization.

The implementation plan and qualification details are in
[DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md](DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md),
[SAGE_OPERATIONAL_CLOSURE_QUALIFICATION.md](SAGE_OPERATIONAL_CLOSURE_QUALIFICATION.md), and
`agent-memory/initiatives/sage.md`. `Data/agent_logs/project_board.md` remains the task-ID and
status authority.

## Open release follow-ups

- Review `feat/TASK-154-server-admin-ui` at `54ceb05` against current `dev`, then integrate it or
  explicitly retire it before packaged-build qualification.
- Qualify the admin review journey against packaged release artifacts.
- Capture and privacy-review the approved launch screenshots and video.
- Record the exact approved Glama listing path and add its score badge to
  `punkpeye/awesome-mcp-servers` PR #13608.

Clean Windows installation and full Managed Linux validation were confirmed complete by the owner
on 2026-09-10 and must not be reintroduced as pending work.
