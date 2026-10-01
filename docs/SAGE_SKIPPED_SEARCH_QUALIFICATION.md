# Sage skipped-action records and full-entry search

TASK-157, 2026-10-01. Branch: `features/TASK-157-sage-parity`.
Reference: `ai_sessions_skills` at `b85a1ab`.

## Behavior

Explicit model skips enqueue durable publication in the same SQLite transaction as the terminal
verdict. Existing skipped jobs are backfilled on schema initialization. `_skipped.md` stores one
hashed signature and a fixed safe explanation per action. This deliberately omits raw arguments
and model-generated skip prose. It uses atomic replacement, rejects unsafe files, and is idempotent
after a write succeeds but SQLite acknowledgement is interrupted. File failures retry without
repeating the model. Model outages create neither skipped records nor knowledge entries.

`sage search <keywords> [--topic <filename substring>]` now returns complete current Markdown
entries with their filename and line. Terms are Unicode case-folded literal substrings, all of
which must match the same entry (including its topic filename). There are no query operators or
wildcards. Heading matches rank above command matches, then filename and line break ties.
`sage topics` lists documented Markdown topics and entry counts. README and underscore-prefixed
files, including skipped records, are excluded from both operations.

Before each search or topic listing, SQLite's derived `sage_entries` cache is transactionally
refreshed from Markdown. Publication filenames are reconciled by hashed signature. Manual edits,
renames, merges, and deletions are reflected on the next query, including after database restart.
Missing content clears the historical filename without silently recreating deleted knowledge.
The older command-family FTS query remains an internal capture API; the CLI does not fall back to
raw capture records when documentation is missing.

## Validation

Executable coverage:

- `TestReferenceSkippedFileRecordsTheSig`, `TestSageSkippedAtomicFailureAndUnsafeFile`.
- `TestReferenceSkipVerdictIsRecordedSoItNeverReturns`: file failure, restart, lost acknowledgement,
  duplicate capture, old-job backfill, and sensitive model-prose exclusion.
- `TestReferenceModelOutageIsNotRecordedAsSkipped` plus existing distiller/worker failure tests.
- `TestReferenceKeywordSearchMatchesWholeEntriesAndFiltersTopics`: whole-entry AND terms,
  punctuation, Unicode folding, topic filtering, and skip/index exclusion.
- `TestSageEntrySearchReconcilesMovesEditsDeletesAndRestart`: publication through merge, manual
  rename/edit, restart, deletion, and database filename reconciliation.
- `TestSageEntrySearchRankingAndFailedSnapshot`: ranking/limit, pending merge, malformed Markdown,
  and transactional cache rollback.
- `TestSageSearchRendersCompleteCurrentEntry`: CLI formatting and argument validation.

Validation passed on Windows: `go test ./... -count=1 -timeout 60s` and `go vet ./...`.
Fedora WSL2 passed focused `-race` tests matching `TestReference|TestSage` and vet for
`internal/sage/knowledge`, `internal/db`, and `internal/infra`.
Ledger: 136 scenarios — 27 implemented, 1 adapted, 12 partial, 96 pending.

## Limits and remaining work

The snapshot is bounded to 4 MiB per file and 64 MiB across topic files. Queries allow at most
4096 bytes and 64 terms, with at most 100 returned entries (CLI: 20). Full refresh favors correctness
for a personal knowledge base; large-library incremental indexing is not qualified.

Participating processes must share the same SQLite database to serialize file work. External
editors do not take that lock; finish saving before searching for a consistent view. Pending merge
recovery and malformed/unsafe files produce an error rather than stale or partial results. Search
does not repair a pending merge; retry the merge first. Parent-directory power-loss durability
remains unqualified. Restoring only Markdown does not restore the SQLite terminal-skip queue.

Safe local Git commits, diagnostics, action equivalence, remaining harness parity, and the repeated
capture-to-knowledge acceptance journey remain open. This is local source qualification, not an
installed-binary, hosted-CI, or upstream-integration claim. Sage never pushes.
