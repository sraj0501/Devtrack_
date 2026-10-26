# Next Steps — DevTrack

_Updated 2026-10-26. This file lists active work only. Completed delivery and validation history
lives in `Data/agent_logs/project_board.md`, release notes, and Git history._

## Immediate repository sequence

Qualify the basic workflow for the owner's upcoming real use case: automatic Git capture,
deterministic ticket association, persisted commits, and complete EOD reports. Time tracking is
removed by product decision and must not return as a requirement.

Fixes and v3.3.0 preparation are pushed to `dev` at `860b91b`. Stable v3.2.0 and the installed host
binary remain separate; rolling development versions use `dev-<commit>`. Packaged container and
isolated Windows no-send checks passed for `dev-dc39101`; they do not qualify the installed host or
real integration delivery.

Follow [testing/basic-workflow.md](testing/basic-workflow.md):

1. Back up and upgrade the native Windows test installation to the qualified development build;
   verify the running version and rehearse capture, branch/ticket context, outages, restart, queue
   review, and EOD with a live server.
2. Verify controlled real email/PM delivery and duplicate prevention using approved test destinations.
3. Reconcile one representative workday's commit hashes and ticket mapping against its report.

Keep the daemon running during capture. Commits made while it was stopped have no qualified
catch-up guarantee; resolve that limitation before depending on it. Scheduled local-date/midnight
behavior and the host's generated hooks also need the planned rehearsal.

## Product boundary

`devtrack sage` is the local cross-harness command-knowledge product, not a repository-chat or
autonomous Git agent. Hooks remain silent, bounded, model-free, and fail-open. SQLite owns private
queues and derived retrieval state; deterministic Markdown is the portable knowledge authority.
Raw harness payloads never enter PostgreSQL, telemetry, MCP, or remote synchronization.

Sage testing is on hold until the basic workflow passes. Its implementation plan and historical qualification details are in
[DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md](DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md),
[SAGE_OPERATIONAL_CLOSURE_QUALIFICATION.md](SAGE_OPERATIONAL_CLOSURE_QUALIFICATION.md), and
`agent-memory/initiatives/sage.md`. `Data/agent_logs/project_board.md` remains the task-ID and
status authority.

## Deferred follow-ups

- Legacy communication-learning adapter repairs and platform event/connector parity remain on hold.
- Resume Sage qualification only after the capture/report acceptance gates pass.
- Reconcile and re-qualify `feat/TASK-154-server-admin-ui` at `54ceb05` as a separate post-v3.2.0
  task; the old branch is explicitly excluded from this release.
- Capture and privacy-review launch screenshots and video from tested packaged behavior.
- Record the exact approved Glama listing path and add its score badge to
  `punkpeye/awesome-mcp-servers` PR #13608.

Earlier Windows installation and Managed Linux validation are historical evidence. The targeted
Windows rehearsal above qualifies the new development build and does not reopen those older gates.
