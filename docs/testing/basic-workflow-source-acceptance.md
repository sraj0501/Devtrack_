# Basic workflow source acceptance record

Updated: 2026-10-22.

The fixes and v3.3.0 preparation were pushed to remote `dev` at `860b91b` on 2026-10-22.
This record separates the final source/container regression from the initial v3.2.0 baseline.
The rolling development release retains its `dev-<commit>` identifier; stable publication and
host installation are separate. Follow [the current test plan](basic-workflow.md) for remaining
packaged-build, native Windows, controlled-delivery, and representative-workday qualification.

**Subsequent owner decision:** remove time tracking entirely because it enables micromanagement.
The work-duration results in the baseline section below are historical evidence, not current
requirements. The development code removes collection/inference, timed sessions and corrections,
work-session HTTP/chat commands, time metrics in report formats/LLM enrichment, and session sync. Ticket context and commit-based
EOD reports remain. Historical local duration tables are retained inert, not read or synchronized.
The former time-correction blocker is superseded by removal. Rapid-commit capture and CLI report
visibility are fixed on `dev`, with validation below. v3.2.0 retains the old behavior
until upgraded.

User priority: prepare for a real use case in a couple of days.

The user explicitly put legacy learning-command repairs and platform event/connector parity on
hold. Qualify Git commits and EOD reports first; move to Sage only after the basics
are reliable. Pre-commit activity capture is outside this test scope. This request authorized
container testing; the subsequent version update and `dev` push were explicitly authorized.
No stable publication, host installation, or board change was performed.

## Time-tracking removal validation

The updated Linux client was built from the working tree and exercised in the same disposable
container stack with the updated server source. Selecting `BASIC-103` on a noncanonical branch,
making a native Git commit, generating EOD, and clearing ticket context passed. The retired
`work adjust` command is rejected. Legacy row counts stayed unchanged across the run:
3 work sessions, 11 evidence records, and 1 historical adjustment. Evidence:
`Data/basic-acceptance/results/no-time-smoke.json`. The test stack was stopped afterwards.

Go tests, `go vet ./...`, and the Linux build passed. Regression tests cover removal of
duration fields from report formats/model schemas and suppression of queued legacy session sync
on upgrade. Legacy database schemas remain for compatibility; their presence does not enable
new collection. No host installation or public package was updated.

Time-removal checkpoint Python suite: **928 passed, 10 skipped**. Run with local dotenv loading disabled
and no PostgreSQL URL; queue tests now explicitly select their temporary SQLite backend/file
instead of inheriting deployment settings. The 10 live-PostgreSQL tests were skipped in this
host run. Logs: `Data/work-time-removal/python-resumed.txt`, `go-final.txt`, and `go-vet.txt`.

## Capture and report visibility fixes

Implemented before the v3.3.0 preparation commit; pushed to `dev`, not installed on the host.

- The Git monitor seeds pre-existing history and enumerates every unseen ancestor oldest first,
  deduplicating shared ancestry. It checks all local branch tips so a branch switch during a slow
  callback cannot hide work or assign it the final HEAD's ticket. Source branches are checked
  before the default branch's merge ancestry. Historical commits cannot trigger auto-enhancement
  of a different HEAD; HEAD and staged changes are rechecked after model processing.
- Authenticated `/queue/history` supplies paginated PostgreSQL history, including completed and
  rejected reports, ordered by creation time/ID rather than confidence expiry. CLI `eod status`,
  `eod show`, queue listing, and counts include server records. Server-owned IDs are shown as
  `server:<id>`; bare numeric IDs remain local. Server approve uses the existing atomic execution
  claim; edit/reject use `/queue/review` and atomically require pending status. Server rows are
  not copied into SQLite, and client-event synchronization remains opt-in.
- An unavailable server is surfaced as unavailable history rather than an empty success. Legacy
  local reports remain readable with a warning during an outage.

Final container regression: **20 rapid commits**, **3 commits on a branch switched away from
before processing completed**, **5 outage commits**, and **1 post-restart commit** all appeared
exactly once. Ticket provenance remained correct. HTTP delivery was deliberately delayed by
600 ms per commit. Recovered EOD included all test work; status/show/queue list/history and
edit/reject passed. Approving a rejected action was denied. A deliberately colliding local ID
was unchanged by server review. A real scheduled EOD also appeared in CLI history and the queue.
Evidence: `Data/basic-acceptance/results/fixes-smoke.json` and `scheduled-fix-smoke.json`.

Full Go suite, vet, and Linux build passed. Full Python suite: **936 passed, 10 skipped**
(unconfigured host live-PostgreSQL test lane); container tests used real PostgreSQL. Logs:
`Data/basic-acceptance/results/go-fixes.txt` and `python-fixes.txt`. One initial Go run hit an
existing Windows Sage worker cleanup file-lock flake; subsequent complete runs passed without
changing Sage. One container harness assertion was corrected to wait for HTTP delivery after
local persistence; it was not a capture failure. Test stack stopped after qualification, volumes
preserved, original `devtrack-isolated` stack left untouched. Real email/PM delivery remains
unqualified because the harness deliberately has no real integration credentials.

## Historical v3.2.0 baseline environment and evidence

The remaining baseline sections describe the original packaged v3.2.0 test before fixes and time
removal. They are retained for reproduction and must not be read as current behavior or requirements.

An isolated Docker Compose project, `devtrack-basic-acceptance`, ran a Linux client, Python
server, and PostgreSQL 16 without real PM/email credentials or host user configuration.
The existing `devtrack-isolated` stack was left alone. The packaged Linux amd64 v3.2.0 client
reported commit `701c7a7`; its archive matched the local release checksum manifest:
`1027389523a19f50962ec92868996da4972ac17d57f78214b631da91981bda91`.
The server used the existing dependency image with current backend/migrations mounted read-only;
those paths matched v3.2.0 at the original baseline. The later regression mounted updated source.
Neither run qualifies a freshly built server image.

Machine-local, ignored evidence is under `Data/basic-acceptance/`: `compose.yml`, `client.env`,
`run.py`, `results/checks.jsonl`, `results/commands.txt`, `results/eod-normal.txt`,
`results/eod-recovered.txt`, and the preserved runtime state. Credentials in this harness are
disposable test values only. Queue polling was set to 3600 seconds to retain staged actions for
inspection; normal automatic execution/delivery was not qualified. Sage capture was disabled.

## Historical v3.2.0 verified behavior

- The installed binary starts and automatically records normally spaced native Git commits.
- Canonical branches map to `BASIC-101`/`BASIC-102`; nonconforming branches remain recorded as unlinked.
- Commit hashes, messages, branch provenance, and privacy-bounded time evidence persist in SQLite.
- Inferred work sessions and confidence are visible through `work status`. These are padded
  estimates, not elapsed-time measurements; rapid ticket switches can receive zero additional minutes.
- A server outage does not prevent commit/time recording. After recovery, an explicit EOD request
  includes the offline commit from local SQLite. Replay of the failed per-commit HTTP trigger was
  not established by this test.
- Restart retains recorded history without duplicating the current HEAD, and captures new work.
- With the LLM unavailable, on-demand EOD generation produces a useful template grouped by ticket,
  including unlinked work, and stages the report in PostgreSQL.
- A real cron EOD fired at 23:02 Asia/Kolkata, generated the report, and stored it pending in
  PostgreSQL. Periodic timer triggers also recorded successfully.
- `work adjust 12` succeeded on retry, preserved the measured duration, and displayed 42m total
  (30m inferred plus 12m adjusted) through `work status`.

## Original functional failures (fixed above; retained as reproduction evidence)

1. **Rapid commits are lost.** Two independent five-commit bursts each captured only three hashes;
   changes 0 and 3 were absent in each run. `internal/infra/git_monitor.go` observes only the latest
   HEAD per check and calls downstream processing synchronously. Intermediate commits are not
   enumerated. Reports consequently omit the same work. Require every expected hash exactly once
   under bursts and slow/unavailable downstream services; retain correct ticket provenance.
2. **Generated reports are invisible to the CLI queue/history.** `eod` returns a real server-side
   action ID, but `eod status` and `eod show` say `No EOD report on record`, and `queue list` says
   `No pending actions`. PostgreSQL independently confirmed the reports were pending. The Go CLI
   readers query local SQLite while the Python queue gateway writes PostgreSQL. Reconcile the
   ownership/read path without bypassing the pending-action boundary or enabling raw data sync.
3. **Superseded by removal: time correction could fail under concurrent activity.** At the scheduled EOD/timer boundary,
   `work adjust 12` returned `audit work session 3 adjustment: database is locked (5)
   (SQLITE_BUSY)`. An idle retry succeeded. The former repair requirement is superseded by removal;
   do not restore time correction to satisfy this historical scenario.

## Baseline test limitations and harness corrections

- Initial hand-written configuration omitted `SEND_ON_TRIGGER`, `SEND_DAILY_SUMMARY`, and
  `TEAMS_MENTION_USER`; startup panicked until these were supplied. The Go auto-accept-terms path
  requires `DEVTRACK_AUTO_ACCEPT_TERMS=1`, not `true`. Guided setup was not tested, so these are
  manual-configuration observations, not evidence that the setup wizard fails.
- The first capture failure occurred before a daemon successfully started and is a harness setup
  failure. It must not be counted as a product capture regression.
- The first scheduled-EOD check lost its process-only schedule setting during the harness restart;
  the subsequent explicitly rescheduled real cron test passed. Do not report a scheduler defect.
- No usable host Ollama endpoint was reachable from the client container. AI-written narrative
  quality, a native Windows workflow, real delivery, daemon-off catch-up, and overnight/timezone
  boundaries remain unqualified. Duration accuracy was part of the original baseline scope and is
  no longer a product or acceptance requirement.

The reproduced lost-commit and report-visibility failures now pass the container scenarios above.
The host installation/public package still need an explicitly authorized upgrade and a real
integration-delivery check before making claims about the user's installed daily workflow.
