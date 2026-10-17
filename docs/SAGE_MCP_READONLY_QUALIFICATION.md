# Sage read-only MCP qualification

TASK-169 / SAGE-005 adds four local Sage tools to DevTrack's existing stdio MCP server. The
implementation is on `features/TASK-169-sage-mcp-readonly` in commit `122c06e`; it is development
work and is not part of stable v3.1.1.

## Contract

| Tool | Input | Result |
| --- | --- | --- |
| `sage_search` | Required literal `query`; optional `topic`; optional `limit` from 1–100 | Complete current Markdown entries with topic, filename, line, heading, body, and command |
| `sage_topics` | None | Current topic names and entry counts |
| `sage_get_entry` | Exact `topic` and positive `line` returned by search | One complete current Markdown entry |
| `sage_status` | None | Capture state, durable pipeline counts, and installed-adapter modes |

All four declarations are read-only, non-destructive, idempotent, and closed-world. They use the
same local SQLite database and deterministic Markdown snapshot as the Go CLI. They do not call a
model, Python, PostgreSQL, a PM provider, or any network service.

## Privacy and consistency

- Search and exact retrieval return published knowledge, not raw capture events or model traces.
- Status returns bounded counts and adapter installation modes. It excludes captured commands,
  raw signatures, logs, prompts, output, reasoning, and local adapter configuration paths.
- Every knowledge read refreshes the derived SQLite cache only after the complete Markdown
  snapshot is readable. Manual edits are visible; moved and deleted entries are not served from a
  stale cache.
- Schemas reject undeclared properties. Required values, string sizes, integer bounds, missing
  entries, and cancelled contexts fail without falling back to raw data.
- The original six DevTrack context tools and local stdio lifecycle remain unchanged. The server
  advertises ten tools on this branch, and `devtrack mcp test` now exercises `sage_status` too.

## Local evidence

The following gates passed on 2026-10-16:

- `go test ./... -count=1` in the Go 1.24 Linux container.
- `go vet ./...` in the same container.
- `go test -race ./internal/mcp ./internal/db -run 'TestSage|TestGetSageEntry|TestRegister|TestReference' -count=1`.
- Native Windows amd64 test binaries for `internal/mcp`, focused Sage database tests, and the main
  package MCP registration tests.
- Focused tests cover all four schemas and annotations, complete-entry search, exact lookup,
  edit/move/delete reconciliation, privacy canaries, adapter errors, invalid inputs, cancellation,
  and preservation of the existing tool surface.

PR #291 targets `dev`. All 19 hosted checks pass on head `8ba7dca`, including Glama container
introspection, five native MCPB platforms, and MCP Registry metadata validation. Integration still
requires explicit authorization.
