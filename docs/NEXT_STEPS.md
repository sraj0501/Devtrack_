# Next Steps — DevTrack Sage

_Updated 2026-09-26. This file lists active work only. Completed delivery and validation history
lives in `Data/agent_logs/project_board.md`, release notes, and Git history._

## Immediate repository sequence

TASK-158 merged into `origin/dev` through PR #264. Its implementation restores the silent Git
contract: normal Git remains native, `devtrack git commit` has no ticket/time/PM/push follow-up,
observation is silent and fail-open, and database initialization no longer leaks into the commit
path. Native Linux plus explicit TTY/non-TTY qualification still remain release gates; this is
unreleased development state, not a claim about v3.1.1.

The PR #263 and PR #264 check sets retain historical failed hosted-Windows unit-test jobs. PR #264's
job hit the 60-second timeout in SQLite-backed `internal/db` and `internal/mcp` tests. PR #265 later
passed all 19 hosted checks, including the full Windows test job, re-establishing a green branch
baseline. TASK-158's native-Linux TTY/non-TTY qualification remains a separate open gate.

The next product task is **TASK-160**, which implements the deterministic branch-to-ticket contract
defined in `PRODUCT_BIBLE.md`: canonical branch, explicit commit prefix/trailer, explicit active
ticket, otherwise unlinked. TASK-159 follows by replacing per-commit duration entry with bounded,
local activity-window inference and optional correction commands. After those product-correctness
slices, continue TASK-157 / SAGE-003 at its durable distillation-queue boundary.
Before that implementation slice, reconcile every reference scenario with an exact Go test and
explicit implementation status in the parity matrix.

TASK-161 owns the rolling `dev` update-channel implementation on
`features/TASK-161-rolling-dev-channel`; PR #265 targets `dev`. The full Go suite, vet, focused
tests, and documentation checks pass locally, and all 19 hosted PR checks pass. The feature still
requires review, integration, and successful first-prerelease publication before public
documentation may describe `devtrack upgrade --dev` as supported or shipped behavior.

## Current product initiative

Complete **DevTrack Sage** as DevTrack's local, cross-harness memory layer. The product is no longer
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
context is maintained in `agent-memory/initiatives/sage.md`. SAGE-001 established the
contract and port boundary as TASK-155. SAGE-002 is complete as TASK-156: the atomic bounded
spool, append-only SQLite event store, daemon importer, and opt-in read-only Codex history adapter
are implemented, hook installation/removal is reversible, and the isolated packaged capture
journey passes. The first SAGE-003 structured-distillation boundary is also merged: shared Go LLM
transport, validated structured drafts, explicit skip versus retryable failure semantics, and a
non-blocking worker. Continue by wiring durable SQLite claims and daemon lifecycle, then produce
deterministic Markdown, routing/correction, merge/refile, skipped-action records, safe local
commits, and parity closure. SAGE-003 remains incomplete until it produces reference-compatible
structured Markdown knowledge and passes the refreshed parity matrix.
Sanitized observed hook fixtures
remain an external harness-validation gate.
The selective harness registry and external plugin boundary are now fixed in
`SAGE_HARNESS_PLUGIN_CONTRACT.md`; new adapters must plug into that boundary rather than adding
global installer switches or harness-specific storage.
`Data/agent_logs/project_board.md` remains the task-ID authority.

The parity inventory accounts for all 136 reference test methods at `b85a1ab`, including the
previously omitted explicit CLI-history capture scenario. Every row still needs exact Go-test
ownership and an implementation status; a reconciled row count alone is not a completion claim.

## Open release follow-ups

- Review `feat/TASK-154-server-admin-ui` at `54ceb05` against current `dev`, then integrate it or
  explicitly retire it before packaged-build qualification.
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
