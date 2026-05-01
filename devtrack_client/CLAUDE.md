# devtrack-cli

Thin HTTP client that proxies user commands to `devtrack-server`.

## Status

**TASK-008 complete.** GitLab CI/CD pipelines added.

## Structure

```
devtrack_client/
  cmd/cli/
    main.go            ← entry point; routes args to CLIClient
  cli_client.go        ← CLIClient HTTP methods (all 9 routes)
  cli_client_test.go   ← unit tests (httptest; no real server needed)
  devtrack             ← Unix compat wrapper → execs devtrack-cli
  devtrack.bat         ← Windows compat wrapper → calls devtrack-cli.exe
  go.mod               ← module: gitlab.com/devtrack3_cloud/devtrack_cli
  go.sum
  .env_sample          ← 4 vars (URL + token + app name + version)
  .gitlab-ci.yml       ← build + release pipeline
```

## Tests

```bash
go test ./...
```

15 unit tests covering all 9 HTTP methods, auth header handling, error responses,
and unreachable-server behaviour. Uses `httptest.NewServer` — no daemon required.

## Build

```bash
go build -o devtrack-cli ./cmd/cli
```

## Required env vars (CLI only)

```
DEVTRACK_SERVER_URL=http://localhost:8765
CLI_APP_NAME=devtrack-cli
DEVTRACK_VERSION=0.1.0
```

## API contract

All request/response types live in `../contract/api.go`.
The server defines routes; the CLI imports and consumes them.

## Server repo

`../devtrack_server/` — Go daemon + Python backend.
See `../contract/api.go` for the full HTTP API surface.
