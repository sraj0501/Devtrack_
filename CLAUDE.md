# DevTrack Workspace

This is the **monorepo root** during the migration phase. Code is physically separated into two folders that will eventually become two independent GitLab repositories.

## Repo layout

```
devtrack_server/    → gitlab.com/devtrack3_cloud/devtrack_server
devtrack_client/    → gitlab.com/devtrack3_cloud/devtrack_cli
contract/           → shared HTTP API types (imported by both)
```

## contract/ — the shared context bridge

`contract/api.go` defines every HTTP route path and request/response struct used between server and client. **Always read this file first when debugging a server-client issue** — it is the single source of truth for the API surface.

```go
// contract/api.go contains:
// - Route path constants (RouteHealth, RouteStatus, ...)
// - Request/response structs for all 9 endpoints
// - AuthHeader constant
```

## devtrack_server/

Go daemon + Python backend. The server binary listens on two ports:
- TCP `IPC_PORT` (default 35893) — internal, Go ↔ Python bridge only
- HTTP `DEVTRACK_SERVER_HTTP_PORT` (default 8765) — external, CLI ↔ server

**Build:**
```bash
cd devtrack_server/devtrack-bin
go build -o devtrack-server .
```

**Test:**
```bash
cd devtrack_server/devtrack-bin
go test ./...
uv run pytest devtrack_server/backend/tests/
```

See `devtrack_server/CLAUDE.md` for full server architecture, config vars, and debugging patterns.

## devtrack_client/

Thin Go CLI binary — proxies commands to devtrack-server over HTTP.
**Not yet implemented** — code arrives in TASK-004 and TASK-005.

See `devtrack_client/CLAUDE.md` for planned structure and env vars.

## Migration status

| Task | Issue | Status |
|------|-------|--------|
| TASK-001-A: Update doc URLs | #89 | Planned |
| TASK-002: ADR document | #90 | Planned |
| TASK-003: HTTP API on server | #91 | Planned |
| TASK-004: cmd/ restructure | #92 | Planned |
| TASK-005: CLI HTTP client | #93 | Planned |
| TASK-006: Config split | #94 | Planned |
| TASK-007: Tests + compat | #95 | Planned |
| TASK-008: GitLab CI/CD | #96 | Planned |
| TASK-001-B: Final GitLab push | #97 | Planned — last step |

## Working across both repos

When debugging a server-client issue:
1. Read `contract/api.go` — understand the full API contract
2. Read `devtrack_server/devtrack-bin/http_api.go` — what the server implements
3. Read `devtrack_client/cli_client.go` — what the client sends

All three files together give complete context. No need to clone two repos.
