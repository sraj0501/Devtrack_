---
name: Project current state
description: Active implementation work, integration state, and unresolved release follow-ups
type: project
---

**Owner decision (2026-10-30):** DevTrack development and qualification are on hold while the owner
decides its next direction. The project is not archived. DevTrack's standalone landing page is restored in `website/` for independent static deployment. Mogrov owns its public wiki, privacy page,
install scripts, and the current combined-site asset snapshot. The docs are prepared at `mogrov.com/wiki/devtrack/`
and the Mogrov download link redirects to the latest GitHub release. Netlify subscription has been cancelled; `mogrov.com` registration stays at Hostinger.
The combined site is prepared locally in the Mogrov repository; publication and DNS switch remain
separate actions. Treat the work below as paused context, not an active execution queue.

**Immediate user priority (2026-10-21):** qualify basic Git capture and EOD reports for an upcoming
real use case before testing Sage. The owner explicitly removed time tracking as micromanagement;
work durations, sessions, corrections, and time metrics are no longer product requirements.
The development code removes runtime time collection, time-related CLI/work-session APIs, chat commands, report metrics,
and session synchronization; legacy tables remain inert for upgrade compatibility. The public
v3.2.0 package still contains the old behavior until a new build is installed.
Legacy learning repairs and platform parity remain on hold. The fixes now enumerate
unseen commits across local branch tips and read server report/queue history over authenticated
HTTP with `server:<id>` review identities. Container regression and full-suite results are recorded
in the linked acceptance documentation; the host installation has not been updated.
See [basic workflow qualification](initiatives/basic-workflow-qualification.md). On 2026-10-22,
the v3.3.0 preparation and fixes were pushed to remote `dev` at `860b91b`; stable publication and host
installation remain separate. Windows metadata and CI candidate bundles target 3.3.0; the rolling
development channel retains its `dev-<commit>` version convention. The packaged `dev-dc39101`
Linux/PostgreSQL acceptance and isolated Windows no-send checkpoint passed on 2026-10-22;
details are in the [acceptance record](../docs/testing/basic-workflow-source-acceptance.md).
Native Windows with a live server, controlled delivery, and a representative workday remain.
The next gates are in [the daily-use test plan](../docs/testing/basic-workflow.md). No board
mutation is included.

**Sage state:** No approved implementation slice remains pending. The current owner-requested
qualification is on hold until basic Git capture and EOD acceptance. Durable runtime boundaries
are in [the Sage initiative](initiatives/sage.md); historical validation belongs in `docs/SAGE_*`.

**Distribution state:** v3.2.0 is the latest stable public release. Five native MCPB bundles and
the official MCP Registry record for `io.github.sraj0501/devtrack` are published.

**Deferred UI work:** The server-admin redesign remains isolated on
`feat/TASK-154-server-admin-ui` at `54ceb05`. It is explicitly deferred from v3.2.0 because the old
branch predates canonical-memory migration and needs a separate drift/UI-acceptance pass.
It does not block qualification of the existing basic Git/report workflow.

**Active distribution follow-ups:** qualify the native installed workflow and controlled delivery,
capture privacy-reviewed media from real behavior, and record the exact Glama path before updating
the score badge. Third-party directory forms remain owner-account actions.

**Known communication-learning gap:** Managed onboarding can seed voice data from local Git
history, but several Python HTTP learning/status/sync/profile adapters are incomplete. Treat those
CLI paths and Teams/Outlook learning as unavailable until repaired.

**Storage boundary:** Go remains SQLite-only. Python requires `POSTGRES_URL`, validates it, and
applies Alembic before serving; client-event sync is opt-in and idempotent.

**MCP boundary:** `devtrack mcp` is local read-only stdio. TASK-169's Sage tools read the current
deterministic Markdown snapshot and bounded SQLite diagnostics. Raw Sage events, logs, prompts,
output, reasoning, and adapter configuration paths never enter MCP, PostgreSQL, telemetry, or
remote synchronization.

**Authority:** `Data/agent_logs/project_board.md` owns task status and IDs; GitHub
`sraj0501/Devtrack_` owns source and releases.
