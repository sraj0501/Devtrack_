# Sage persistent routing qualification

TASK-157 checkpoint, 2026-10-01. This is local routing evidence, not full Sage acceptance.

## Implemented

- `devtrack sage routes` lists effective destinations and their source as JSON.
- `devtrack sage route <binary> <topic>` remembers a destination for future entries.
- Existing Markdown entries take precedence over remembered routes, including after manual
  file moves. If multiple files document a binary, the first filename in lexical order wins.
- Remembered destinations survive process restart in `.routes.json` under the configured
  knowledge directory. Unknown binaries fall back to their own topic files.
- Publication uses these destinations. A route correction does not move existing entries;
  existing entries still take precedence. Merge/refile is the next implementation slice.
- Route changes atomically replace the file and share the SQLite writer lock with publication.
  Different processes must use the same Sage database to coordinate writes.
- Malformed route data fails closed without being overwritten. Files remain bounded by the
  writer's 4 MiB limit and regular-file/path checks. README and underscore-prefixed Markdown
  files do not supply routes. No commands, models, network requests, or Git operations run here.

## Validation

On 2026-10-01, the full Windows Go suite and `go vet ./...` passed. Fedora WSL2 passed
`go test -race ./internal/sage/knowledge ./internal/db ./internal/infra -run 'TestReference|TestSage' -count=1 -timeout 60s`
and vet for those packages. Tests cover persistence after reopening the database/writer,
publication following corrections, manual moves, stale-route precedence, malformed data,
atomic replacement failure, and a concurrent correction waiting for publication's database lock.

Nine routing reference scenarios now have executable Go tests in `routes_test.go`; the parity
ledger contains 22 implemented, 15 partial, and 99 pending scenarios. A consistent ledger is
not proof of complete runtime parity.

## Remaining limits

Routing currently extracts simple command binaries. Shell wrappers, compound commands,
action equivalence, fuzzy section matching, classifier behavior, merge/refile, skipped-action
records, and complete diagnostics remain separate parity work. Search/topics still expose
the existing command-family index; route changes do not yet reconcile full-entry search.
Safe local knowledge commits and the complete twice-run capture-to-knowledge journey remain
open. This checkpoint does not qualify a newly installed binary or hosted CI.
