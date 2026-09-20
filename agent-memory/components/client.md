---
name: DevTrack client
description: Go client build, architecture, configuration, and server boundary
type: project
---

# devtrack_client — Client Binary

Go module: `github.com/sraj0501/Devtrack_/devtrack_client`

This is the **canonical** Go source for the `devtrack` binary. (The legacy `devtrack-bin/` mirror was removed in TASK-048.)

See `../INDEX.md` and `../operations/build-guide.md` for shared project context, configuration
patterns, and vision rules.
See `docs/ARCHITECTURE.md` for the HTTP/JSON boundary between this client and `devtrack_server`.

## Build & Run

```bash
cd devtrack_client
go build -o devtrack .          # Build the devtrack binary
go test ./...                   # Run all Go tests
go vet ./...                    # Run Go linter
go generate                     # Regenerate resource_windows_amd64.syso (Windows icon + version)
```

Cross-compile (all targets use CGO_ENABLED=0):

```bash
GOOS=linux   GOARCH=amd64   CGO_ENABLED=0 go build -o devtrack_linux_amd64 .
GOOS=linux   GOARCH=arm64   CGO_ENABLED=0 go build -o devtrack_linux_arm64 .
GOOS=darwin  GOARCH=amd64   CGO_ENABLED=0 go build -o devtrack_darwin_amd64 .
GOOS=darwin  GOARCH=arm64   CGO_ENABLED=0 go build -o devtrack_darwin_arm64 .
GOOS=windows GOARCH=amd64   CGO_ENABLED=0 go build -o devtrack_windows_amd64.exe .
```

## Architecture

The client is a single Go binary. `main.go` and the `cli*.go` files live at the package root;
core runtime is split into layered `internal/` packages (acyclic):
`config` · `db` · `health` · `learning` ← `trigger` ← `infra` ← `daemon`; plus `trigger` ← `tui`.
Also: `connectors/{github,gitlab,azure}/` (Go-native PM connectors), `internal/alerts/` +
`internal/notify/` (Go-native ticket alerts + notifiers), `internal/telegram/` (Go-native bot),
`internal/sage/` (capture and command knowledge), and `internal/gitcmd/` (independent Git/commit
helpers). See `../operations/build-guide.md` for the full package map.

The `git` subcommand is handled Go-natively through `internal/gitcmd` — no bash or Python wrapper.
Windows syscall isolation uses build-tag splits (`*_unix.go` / `*_windows.go`).

## DevTrack Sage

DevTrack Sage is the Go-native cross-harness, self-writing command-knowledge product. Its current
implementation boundary and ordered work are in `../initiatives/sage.md`. The legacy repository
agent and `gitsage/` package are removed; do not restore their chat, autonomous Git, or free-form
question behavior as Sage architecture.

## Configuration

All env vars consumed by the Go client are listed in `.env_sample`. Every accessor is in `config_env.go` — no `os.getenv` calls anywhere else in Go code. Missing vars panic with a clear message at startup.

Key client-only vars: `IPC_CONNECT_TIMEOUT_SECS`, `HTTP_TIMEOUT`, `HTTP_TIMEOUT_SHORT`, `HTTP_TIMEOUT_LONG`, `IPC_RETRY_DELAY_MS`.

## Server Communication

The client uses the documented authenticated HTTP/JSON boundary to call `devtrack_server`, with
`/trigger/*` as the primary event family. The server URL is set via `DEVTRACK_SERVER_URL`. In
managed mode the client spawns the server as a subprocess.

Full API contract: `docs/ARCHITECTURE.md`

Auth: `X-DevTrack-API-Key` header (value from `DEVTRACK_API_KEY` env var).
