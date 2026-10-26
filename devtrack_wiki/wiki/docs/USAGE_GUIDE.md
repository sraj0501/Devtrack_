# Usage guide

Stable v3.2.0 remains the public release. The no-time-tracking behavior and direct server-history
review described here are v3.3.0 development changes on `dev`; rolling binaries use `dev-<commit>`.

## Daily workflow

1. Configure a repository with `devtrack setup` or `devtrack workspace add`.
2. Name branches with a ticket ID, for example `feature/PROJ-123-description`.
3. Start the daemon once with `devtrack start` or install autostart.
4. Commit normally. The daemon observes without prompting.
5. Review staged work with `devtrack queue list` or `devtrack tui`.
6. Generate the day's narrative with `devtrack work report` or `devtrack eod generate`.
   Both call the Python server and attempt to stage a report; these are not read-only previews.
   Use `devtrack eod show` to read an existing report without generating another.

## Signal priority

DevTrack resolves ticket context from a full canonical branch match, an explicit first-line commit
prefix or `Refs:` trailer, or an explicit `devtrack work start`—in that order. Otherwise the commit
is unlinked. Incidental prose IDs, recent tickets, and LLM suggestions are never authoritative.
Conflicts are retained for correction and cannot stage outbound work. None of these states blocks
Git.

```bash
devtrack ticket convention
devtrack ticket check feature/PROJ-123-description
devtrack ticket link abc1234 PROJ-123
```

## Optional interactive Git wrapper

`devtrack git commit` can refine the message before Git runs. After a successful commit it returns
without asking about tickets, time, PM posting, or pushing. This explicitly invoked helper is
separate from the silent daemon path.

## Pending actions

```bash
devtrack queue list
devtrack queue approve <id>
devtrack queue edit <id> '<json>'
devtrack queue reject <id>
devtrack queue flag <id> "correction"
```

Every external PM, email, or Git action must pass through this queue and carry confidence.
Eligible actions can advance under configured earned auto-approval rules after their review window;
inspect the queue promptly when a destination is enabled.
Development builds list local and server history directly, without requiring opt-in event sync.
Local IDs are numeric; preserve `server:<id>` when approving, editing or rejecting a server row.
Server edits/rejection require a still-pending row, and server flagging is unsupported. During a
server outage, listing can still display local rows with a warning; `queue status` returns an error
instead of partial counts.
`queue list --all` shows the last 24 hours across statuses; it is not an unlimited-history view.
The client TUI reads local SQLite actions; use the CLI for server-qualified rows.

## Work and reports

DevTrack reports work without measuring time spent on each task. Active-ticket selection is
optional context only. It creates no timed session or duration record.

```bash
devtrack work start PROJ-123
devtrack work status
devtrack work stop
devtrack work report
devtrack eod generate
devtrack eod show
devtrack eod status
```

The running monitor retains rapid commit bursts and unseen commits on local branches switched
away from during processing. Startup establishes existing history as a baseline, so do not depend
on stopped-daemon catch-up. `eod show` and `eod status` select the latest local/server report.
Generation can return narrative text even when queue staging fails. Confirm the
`Queued as action server:<id>` output from `eod generate` and inspect the queue before assuming
a report was saved or scheduled for delivery. `work report --email <address>` supplies a destination.
The packaged `dev-dc39101` build passed isolated Linux/PostgreSQL and Windows no-send checks.
Native installed Windows, real delivery and representative-workday acceptance remain the next
gates; see `docs/testing/basic-workflow.md` in the repository. Sage testing is on hold.

## Agent context

```bash
devtrack mcp setup
devtrack mcp test
```

MCP reads local SQLite and does not require the Python server.

## Operations

```bash
devtrack status
devtrack doctor
devtrack logs -f
devtrack pause
devtrack resume
devtrack autostart-install
```

Run `devtrack help` for the current command surface.
