---
name: Basic workflow qualification
description: Current daily-use acceptance scope, remaining gates, and deferred work
type: project
---

# Basic workflow qualification

The immediate owner priority is reliable automatic Git capture, ticket association, persisted
commits, and EOD reporting for an upcoming real use case. Pre-commit activity is outside scope.
Time tracking was removed by owner decision because it enables micromanagement. Do not restore
duration inference, timed sessions, corrections, duration metrics, or session synchronization.
`work start`/`stop` select and clear ticket context; legacy duration tables remain inert for
upgrade compatibility.

## Current boundary

The fixes and v3.3.0 preparation are on remote `dev` at `860b91b`. Development packages retain
the `dev-<commit>` version convention. This does not update the installed host binary or publish
a stable v3.3.0 release; v3.2.0 remains the last recorded stable release.

The Git monitor scans unseen ancestry across local branch tips while running. Startup seeds
existing history, so commits made while the daemon is stopped must not be assumed to be captured.
Server report/queue readers use authenticated HTTP history and `server:<id>` review identities;
bare numeric action IDs remain local. Report reads do not mirror server rows into SQLite or
enable optional event synchronization. Queue "today" counts use UTC, whereas EOD commit selection
uses the local calendar day. Test those boundaries separately; queue status errors on unavailable
server history, while list/show can display available local rows with a warning.
The local "Posted today" count includes approved rows, so it cannot prove delivery. Atomic
pending-only edit/reject applies to server review; local review must be qualified independently.

## Remaining acceptance gates

Follow [the daily-use test plan](../../docs/testing/basic-workflow.md), in order:

1. Rehearse native Windows capture with the qualified packaged build, ticket mapping, restart,
   server outage, report review, and the scheduled local-date boundary. Inspect the generated Git
   hook for obsolete host paths.
2. Verify controlled email/PM delivery, pending-action review, rejection, and duplicate prevention,
   plus narrative accuracy with the intended model and its unavailable-model fallback.
3. Complete one representative workday and reconcile expected commit hashes/tickets with EOD.

Daemon-off catch-up, overnight boundaries, native installation, real delivery, and AI-written
narrative quality remain unqualified. Source/container validation is recorded in
[the acceptance record](../../docs/testing/basic-workflow-source-acceptance.md), not evidence that
the user's installed workflow has been upgraded or qualified.
The packaged `dev-dc39101` Windows binary passed an isolated no-send capture, hook-path,
ticket-context, and restart checkpoint on 2026-10-22. The matching packaged Linux binary then
passed the disposable PostgreSQL rapid/branch-switch/outage/restart, report-review, scheduled EOD,
and no-time-tracking scenarios. The same acceptance record has checksums and remaining gates.

## Deferred work and authorization

Sage qualification, legacy communication-learning repairs, and platform event/connector parity
remain on hold until the basics are accepted. Keep the existing isolated stack untouched when
using the disposable `devtrack-basic-acceptance` environment. The latter is stopped with volumes
preserved; ignored local artifacts under `Data/basic-acceptance/` are not portable test fixtures.
Real integration writes, host upgrades, stable publication, and board changes require their
relevant user authorization. The documentation role does not authorize Git or publication actions.
