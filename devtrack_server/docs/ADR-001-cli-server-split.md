# ADR-001: CLI / Server Split

**Status:** Accepted  
**Date:** 2026-05-01  
**Issues:** #90 (ADR), #91–#95 (implementation tasks)

---

## Context

`devtrack-bin` is currently a single binary that acts as both the background daemon (server) and the user-facing CLI. When a user runs `devtrack start`, the binary forks itself into a background process. When they run `devtrack status`, the same binary reads the PID file and queries the daemon directly — in-process, no network hop.

This works fine on a single machine but creates two problems:

1. **Distribution:** There is no way to install a lightweight "just the CLI" package. Every install pulls the full daemon, Python dependencies, SQLite, and all integrations.
2. **Remote access:** You cannot run the daemon on a server (CI runner, remote dev box, shared team instance) and control it from a laptop.

## Decision

Split the single binary into two independent binaries communicating over HTTP:

```
devtrack-cli  ──HTTP──▶  devtrack-server  ──TCP IPC──▶  python_bridge.py
  (thin)                    (daemon)                      (Python backend)
```

The existing TCP IPC channel between the Go daemon and the Python bridge is **unchanged** — it remains an internal implementation detail of the server.

## Communication protocol

**HTTP** (not raw TCP, not gRPC, not the existing IPC protocol).

Reasons:
- Trivially testable with `curl` — no custom framing needed
- Standard tooling for auth, TLS, timeouts
- Keeps the CLI decoupled from the internal JSON-newline IPC framing that the Python bridge depends on
- Easy to document with OpenAPI later

**Port:** `DEVTRACK_SERVER_HTTP_PORT` (default `8765`). Added to `.env_sample` as a required variable.

**Auth:** Optional `X-DevTrack-Token` header. When `DEVTRACK_API_TOKEN` is blank, auth is skipped — safe for localhost. Required only when the server is exposed on a network interface other than loopback.

## API surface

All types live in `contract/api.go` at the monorepo root — the single source of truth imported by both the server and the CLI.

| Method | Route | Action |
|--------|-------|--------|
| GET | `/health` | Liveness check |
| GET | `/status` | Daemon status (running, PID, uptime, paused) |
| GET | `/logs` | Recent log lines (`?tail=N`, default 50) |
| GET | `/version` | Binary version + Go version |
| POST | `/start` | Start the daemon |
| POST | `/stop` | Stop the daemon |
| POST | `/pause` | Pause git monitoring |
| POST | `/resume` | Resume git monitoring |
| POST | `/trigger/force` | Force an immediate trigger (replaces SIGUSR2 path) |

## CLI defaults

```
DEVTRACK_SERVER_URL=http://localhost:8765
```

The CLI requires exactly three env vars: `DEVTRACK_SERVER_URL`, `CLI_APP_NAME`, `DEVTRACK_VERSION`. It does not read any server-internal vars (`IPC_PORT`, `DATABASE_DIR`, timeouts, etc.).

## Backward compatibility

- `devtrack-server` binary is a direct rename of the current `devtrack` binary — zero behavior change for existing users who run the daemon directly.
- A `devtrack-compat.sh` shim routes `devtrack <cmd>` → `devtrack-cli <cmd>` so existing shell scripts and aliases keep working during the transition.
- New `.env` var `DEVTRACK_SERVER_HTTP_PORT` is required — existing deployments will get a clear startup error pointing to `.env_sample`.

## Repository split

| Folder (monorepo) | GitLab repo | Contains |
|---|---|---|
| `devtrack_server/` | `gitlab.com/devtrack3_cloud/devtrack_server` | Go daemon + Python backend |
| `devtrack_client/` | `gitlab.com/devtrack3_cloud/devtrack_cli` | Thin CLI binary |
| `contract/` | Lives in server repo under `contract/` | Shared HTTP API types |

The CLI imports the contract package as a Go module dependency:
```
require gitlab.com/devtrack3_cloud/devtrack_server/contract v0.x.x
```

During the monorepo phase, a `replace` directive points at the local `../contract` path.

## Consequences

**Good:**
- CLI can be distributed as a single ~5 MB static binary with no Python or SQLite dependency
- Server can run headlessly on a remote machine; CLI controls it from anywhere
- API contract is compiler-enforced — type mismatches are caught at build time, not runtime
- Each binary can be versioned and released independently

**Trade-offs:**
- Adds `DEVTRACK_SERVER_HTTP_PORT` as a new required env var — existing `.env` files must be updated
- `devtrack-cli start` no longer starts the daemon directly; users must run `devtrack-server start` or use a process manager
- The force-trigger command loses its SIGUSR2 path for remote CLIs (replaced by `POST /trigger/force`)

## Implementation sequence

1. **TASK-003** — Add `HTTPAPIServer` to `devtrack_server/devtrack-bin/` (9 routes, optional auth, `go test` green)
2. **TASK-004** — Restructure into `cmd/server/` + `cmd/cli/` within the monorepo
3. **TASK-005** — Implement `CLIClient` and full `devtrack-cli` command dispatch
4. **TASK-006** — Split `config_env.go` into server-only and CLI-only files
5. **TASK-007** — Integration tests, `devtrack-compat.sh`, `docs/MIGRATION.md`
6. **TASK-008** — GitLab CI/CD pipelines for both repos
7. **TASK-001-B** — Final push: split monorepo into two GitLab repos
