# devtrack-cli

Thin HTTP client that proxies user commands to `devtrack-server`.

## Status

**Not yet implemented.** Code will be created in TASK-004 and TASK-005.
See root `CLAUDE.md` for the full migration plan and issue links.

## Planned structure

```
devtrack_client/
  cmd/cli/
    main.go          ← entry point
  cli_client.go      ← CLIClient HTTP methods
  go.mod             ← module: gitlab.com/devtrack3_cloud/devtrack_cli
  .env_sample        ← 3 vars only (see below)
  .gitlab-ci.yml     ← build + release pipeline
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
