# Sage operational closure qualification

TASK-170 completes the approved local Sage operations surface on
`features/TASK-170-sage-operational-closure`. The branch is stacked on the fully green TASK-169
head `bf963dd`; it targets `dev` and is not part of stable v3.1.1.

## Command contract

```text
devtrack sage retention --days N [--apply]
devtrack sage quarantine list
devtrack sage quarantine purge <opaque-id> --yes
devtrack sage quarantine purge --all --yes
devtrack sage delete-topic <topic> --yes
devtrack sage repair
devtrack sage export <new-directory>
devtrack sage import <directory>
```

- Retention is a dry run unless `--apply` is present. It removes only normalized source events
  older than the UTC cutoff and their source links; durable summarized knowledge, publications,
  jobs, and Markdown remain.
- Quarantine listing is capped at 500 entries and returns only a 16-character opaque ID, size,
  modification time, and one fixed reason. It never returns a filename, event contents, decoder
  text, command, path, prompt, output, or reasoning. Purge requires `--yes`; a link is removed as a
  link and is never followed.
- Delete accepts one exact portable topic plus `--yes`. It removes routes pointing to that topic,
  repairs the generated index, and rebuilds the derived SQLite retrieval cache from Markdown so a
  deleted topic is not resurrected.
- Repair recovers interrupted merge state, rejects unsafe or oversized Markdown, repairs only
  Sage's generated topic-index block, and deterministically rebuilds SQLite retrieval without a
  model or network call.
- Export creates a new deterministic Markdown-only directory. Import validates the complete source
  first, ignores its generated README, adds only missing portable Markdown, treats byte-identical
  repeats as no-ops, rejects conflicts before writing, and repairs the destination index/cache.

## Safety boundary

Filesystem-changing maintenance requires `devtrack sage pause` first. Operations serialize with
SQLite publication, reject traversal, symlinked files/directories, overlapping import/export roots,
invalid flags and ranges, and use bounded reads plus atomic replacement. Delete and import restore
their prior files when an injected later write fails. A context canceled before maintenance begins
does no work; after the serialized filesystem section starts, it completes consistently rather
than leaving Markdown and SQLite half-reconciled. Resume capture explicitly when maintenance ends.

Markdown is the portable retrieval authority. SQLite entries are a derived cache. All operations
are local and model-free, preserve unrelated files and user prose, and never push or synchronize
raw Sage activity.

## Local qualification

The following gates pass from the current branch with Go 1.24 in `golang:1.24-bookworm`:

- `go test ./...`
- `go vet ./...`
- `go test -race ./internal/db ./internal/sage ./internal/sage/knowledge .`
- focused retention, quarantine, repair, delete, export/import, strict-argument, pause,
  cancellation, idempotency, conflict-preflight, rollback fault, traversal, and symlink tests
- cross-compiled Windows/amd64 test binaries executed natively for the root client, database,
  Sage runtime, and knowledge packages; link-creation cases skip when Windows developer-mode
  privilege is unavailable, while the same cases pass in Linux

Two docu-agent reconciliation passes cover the board, canonical memory, README, wiki, and
qualification record. Hosted required checks and PR review remain the final branch gate.
