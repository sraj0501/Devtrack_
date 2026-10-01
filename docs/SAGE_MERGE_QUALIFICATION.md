# Sage merge/refile qualification

TASK-157 checkpoint, 2026-10-01. Local implementation evidence; full Sage acceptance remains open.

## Behavior

`devtrack sage merge <source> <destination>` moves Markdown entries between topic files,
preserving entry prose, commands, signatures, and section placement. Existing destination
entries remain. Remembered routes pointing to the source, and binaries found in moved entries,
are redirected to the destination. The source is removed and the managed README index rebuilt.
Repeating a completed merge or merging a missing file returns zero; merging a file into itself
is a no-op. Filenames use the existing portable sanitizer.

The command shares publication's SQLite writer lock. A bounded `.merge.json` write-ahead record
contains before/after snapshots. Destination and route replacement precede source deletion.
An interrupted merge is replayed by the next merge, publication, route correction, or signature
update. Replaying the snapshots does not append entries twice. Read-only route listing reports
pending recovery instead of returning an intermediate route view.

Recovery compares files with recorded snapshots and refuses to overwrite later edits. If a
conflict is reported, preserve copies of the edited files and recovery record before resolving
the differences; do not delete the record just to suppress the error. Markdown remains available
for manual inspection. A dedicated recovery/diagnostic command remains future work.

## Safeguards and limits

- Source files containing notes outside parsed entries are rejected instead of losing those
  notes on deletion. This is stricter than the reference, which deletes the source unconditionally.
- Unclosed code fences, unsafe file types/paths, malformed journals, and oversized files fail.
  The journal itself has the writer's 4 MiB limit, so some large combined merges are rejected
  before topic mutation even when individual files fit within that limit.
- All participating processes must share the Sage database. Concurrent external editor writes
  during an individual filesystem operation are not locked. Snapshot checks protect edits made
  between an interruption and recovery; they are not an OS-wide editor lock.
- Atomic replacement and replay cover tested process interruptions and I/O failures. Parent
  directory durability across sudden power loss is not qualified.
- Full-entry search reconciliation and publication-history filename updates remain open;
  existing SQLite command-family search is unchanged. Markdown entries and routes determine
  future publication destinations. No Git commits, pushes, models, or network calls are added.

## Evidence

On 2026-10-01, the full Windows Go suite and vet passed. Fedora WSL2 focused knowledge/database/
infra Sage and reference tests passed with the race detector; vet passed for those packages.
Regression tests cover each journal/topic/routes/index write failure, a fresh writer recovering
without duplicates, recovery conflicts preserving edits, source-note protection, invalid
journals, and publication recovering an interrupted merge. A database restart test confirms
later publication follows the merged destination. CLI tests cover dispatch and argument rejection.

Two pinned reference scenarios now have executable merge tests. The ledger contains
24 implemented, 15 partial, and 97 pending scenarios. Hosted CI, packaged binaries, and the
complete twice-run capture-to-knowledge acceptance journey have not been qualified here.
