# devtrack_client — Client Binary

Go module: `gitlab.com/devtrack3_cloud/devtrack_client`

This is the **canonical** Go source for the `devtrack` binary. The monorepo's `devtrack-bin/` directory is a legacy mirror being retired in TASK-048. All new Go development goes here.

See the monorepo `CLAUDE.md` for full project context, configuration patterns, and vision rules.
See `docs/HTTP_API.md` for the HTTP/JSON boundary between this client and `devtrack_server`.

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

The client is a single Go binary that wires together:

| File | Purpose |
|---|---|
| `main.go` | Entry point; routes CLI args or delegates `git` subcommand to shell wrapper |
| `cli.go` | All CLI command implementations (`start`, `stop`, `status`, `logs`, etc.) |
| `daemon.go` | Lifecycle management (PID file, signals, webhook server subprocess) |
| `integrated.go` | `IntegratedMonitor` — wires together git monitor, scheduler, and IPC server |
| `git_monitor.go` | fsnotify-based Git repository watcher; fires `commit_trigger` on new commits |
| `scheduler.go` | Cron-based periodic trigger using robfig/cron; fires `timer_trigger` |
| `ipc.go` | TCP IPC server (Go side); JSON-delimited messages, one handler per `MessageType` |
| `database.go` | SQLite via modernc.org/sqlite; stores trigger history and task updates |
| `config.go` | YAML config struct (`Data/configs/config.yaml`); all runtime values via `config_env.go` |
| `config_env.go` | All `.env` key accessors for Go — the single source of truth for env var names |
| `http_trigger.go` | HTTPS POST client that sends trigger payloads to `devtrack_server` |
| `learning.go` | Personalized AI learning consent and profile management |
| `cli_boardroom.go` | `devtrack boardroom` command — calls `/boardroom` on the server |
| `cli_plan.go` | `devtrack plan` command — calls `/plan` on the server |
| `versioninfo.json` | Windows binary version metadata for `goversioninfo` |
| `resource_windows_amd64.syso` | Pre-built Windows resource object (icon + version info); `_windows_amd64` constrains it to Windows/amd64 only |

Platform-split files (syscall isolation):

| File | Purpose |
|---|---|
| `daemon_unix.go` | Unix-only daemon signal handling |
| `daemon_windows.go` | Windows stub for daemon signal handling |
| `cli_unix.go` | Unix-only CLI syscall sites |
| `cli_windows.go` | Windows stubs for CLI syscall sites |

## git-sage (bundled Python tool)

`git_sage/` contains the Python git agent. It is **client-owned** — it runs on the developer's machine alongside the binary, not on the server.

```bash
python -m backend.git_sage do "squash my last 5 commits"
python -m backend.git_sage ask "how do I rebase onto main?"
```

The `gitsage/` subdirectory contains the Go shim that invokes the Python module.

## Configuration

All env vars consumed by the Go client are listed in `.env_sample`. Every accessor is in `config_env.go` — no `os.getenv` calls anywhere else in Go code. Missing vars panic with a clear message at startup.

Key client-only vars: `IPC_CONNECT_TIMEOUT_SECS`, `HTTP_TIMEOUT`, `HTTP_TIMEOUT_SHORT`, `HTTP_TIMEOUT_LONG`, `IPC_RETRY_DELAY_MS`.

## Server Communication

The client sends triggers to `devtrack_server` over HTTPS POST. The server URL is set via `DEVTRACK_SERVER_URL`. In managed mode the client spawns the server as a subprocess.

Full API contract: `docs/HTTP_API.md`

Auth: `X-DevTrack-API-Key` header (value from `DEVTRACK_API_KEY` env var).
