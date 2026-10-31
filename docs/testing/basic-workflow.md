# Basic workflow test plan

Scope: automatic Git capture, ticket association, persisted work, and EOD reports. No time
tracking. Sage, communication-learning repairs, and platform parity remain on hold.

Starting point: fixes and v3.3.0 preparation are on `dev` at `860b91b`. Rolling development
packages use `dev-<commit>`; the installed host has not been upgraded. The
[acceptance record](basic-workflow-source-acceptance.md) covers the passing packaged
`dev-dc39101` Linux/PostgreSQL regression and isolated Windows no-send checkpoint. The remaining
gates below qualify the installed daily workflow.

## Completed checkpoint: packaged development build

The packaged `dev-dc39101` Linux/PostgreSQL container gate and isolated Windows no-send gate passed.
The [acceptance record](basic-workflow-source-acceptance.md) contains the artifact versions, hashes,
configuration boundary, and scenario results. Repeat this gate only for a different build.

## 1. Run a native Windows rehearsal

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

## 2. Check real delivery with a controlled destination

- Configure one designated test ticket/project and one test email recipient, using local secrets.
- Generate one ticket update and one EOD delivery action. Inspect destinations and content in
  the pending queue, then explicitly approve these controlled test actions.
- Verify receipt in the destination, matching content, final queue status, and no duplicate
  delivery on a repeated approval or restart. Verify rejection prevents delivery.
- Check AI-written report accuracy with the intended LLM provider; retain the unavailable-model
  fallback check as well.

## 3. Complete one representative workday

Run the qualified installation during normal Git work. Keep a simple expected commit/ticket list
and compare it with the scheduled EOD. Record missing or duplicate work, wrong ticket links,
incorrect report content, delivery failures, and restart/outage behavior.

Accept the basics when the required scenarios have no missing or duplicate commits, correct
ticket provenance, complete readable reports, successful controlled delivery, and no new time
tracking. Resolve any daemon-off catch-up limitation before relying on capture while stopped.
Keep an evidence record of the tested commit and results before starting Sage qualification.
