# Basic workflow test plan

Scope: automatic Git capture, ticket association, persisted work, and EOD reports. No time
tracking. Sage, communication-learning repairs, and platform parity remain on hold.

Starting point: fixes and v3.3.0 preparation are on `dev` at `860b91b`. Rolling development
packages use `dev-<commit>`; the installed host has not been upgraded. The
[source acceptance record](basic-workflow-source-acceptance.md) covers the passing 29-commit
container regression. The gates below qualify the distributable and real daily workflow.

## 1. Qualify the packaged development build

- Wait for the client/server CI and rolling development release for the pushed commit to pass.
- Download that build, verify its checksum and reported commit, and pair it with the matching
  server source. Record OS, version, commit, timezone, and configuration without secrets.
- Run the isolated Linux container scenarios with PostgreSQL using the packaged binary rather
  than the source-built binary: rapid commits, branch switches, server outage/recovery, restart,
  on-demand EOD, scheduled EOD, queue edit/reject, and colliding local/server action IDs.
- Compare expected commit hashes against persisted records: every hash exactly once, correct
  ticket or explicitly unlinked, and every expected work item represented in the report.
- Confirm retired time commands reject requests, new reports contain no duration metrics, and
  historical duration tables receive no new records or synchronization.
- Test `work start <ticket>`, `work status`, and `work stop` as supported ticket-context commands;
  `work adjust` is the retired duration command. Do not treat selecting a ticket as a timer.

## 2. Run a native Windows rehearsal

- Preserve the current executable, configuration, and local database for rollback. Upgrade the
  test installation to the qualified development build and verify the actual running version.
- Use a disposable Git repository and test tickets. Exercise canonical ticket branches,
  explicit ticket context, unlinked work, rapid commits, branch switching, and daemon restart.
- Inspect the generated Git hook and confirm its paths are valid on Windows. The repository's
  existing local post-commit hook emitted an obsolete Linux log-path warning during preparation;
  the commit succeeded, but native hook setup still needs this check.
- Stop only the server, make commits, restore it, and verify local recording and recovered EOD.
  Separately stop the daemon, make a commit, and restart it: document whether catch-up occurs;
  this behavior has not been qualified and must not be assumed.
- Generate and inspect an EOD report. Confirm status/show/history agree and pending actions can
  be edited or rejected without changing unrelated local actions.
- Compare `eod status`, `eod show`, `queue list`, `queue list --all`, and `queue status` while the
  server is available and unavailable. The default list includes pending rows of any age; `--all`
  displays rows created in the last 24 hours. Counts labelled "today" use UTC, so compare them
  separately from reports for the local calendar day. Edit replaces the entire action payload;
  preserve required fields and use `server:<id>` for server-owned rows.
- Check actual action status and destination receipt when confirming delivery: the local portion
  of `queue status` counts both approved and posted rows under "Posted today". A summary count
  alone is not delivery evidence. Test local and server review separately; only server edit/reject
  currently have the documented atomic pending-status guard.
- Verify server-generated reports through the CLI or server history, since the current TUI queue
  reads local actions only. Use `eod show` for read-only inspection; `eod generate` and `work report`
  request report generation and can create a new pending action. Confirm a saved action ID before
  treating printed narrative as successful staging.
- Exercise the configured scheduled EOD across the local date boundary. Verify the intended
  reporting date, commit coverage, and absence of duplicate scheduled actions.

## 3. Check real delivery with a controlled destination

- Configure one designated test ticket/project and one test email recipient, using local secrets.
- Generate one ticket update and one EOD delivery action. Inspect destinations and content in
  the pending queue, then explicitly approve these controlled test actions.
- Verify receipt in the destination, matching content, final queue status, and no duplicate
  delivery on a repeated approval or restart. Verify rejection prevents delivery.
- Check AI-written report accuracy with the intended LLM provider; retain the unavailable-model
  fallback check as well.

## 4. Complete one representative workday

Run the qualified installation during normal Git work. Keep a simple expected commit/ticket list
and compare it with the scheduled EOD. Record missing or duplicate work, wrong ticket links,
incorrect report content, delivery failures, and restart/outage behavior.

Accept the basics when the required scenarios have no missing or duplicate commits, correct
ticket provenance, complete readable reports, successful controlled delivery, and no new time
tracking. Resolve any daemon-off catch-up limitation before relying on capture while stopped.
Keep an evidence record of the tested commit and results before starting Sage qualification.
