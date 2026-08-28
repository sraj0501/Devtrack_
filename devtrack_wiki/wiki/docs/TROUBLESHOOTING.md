# Troubleshooting

Start with the executable diagnostics:

```bash
devtrack status
devtrack doctor
devtrack settings
devtrack logs -f
```

## Managed bootstrap failed

`status` and `doctor` show the active step, last error, bootstrap log path, and recovery command.
Retry idempotently with:

```bash
devtrack doctor --repair
```

Go-native monitoring, queue, MCP, and scheduling remain available while Python is degraded.

## PostgreSQL unavailable

The Python server requires `POSTGRES_URL` and does not fall back to server-side SQLite.

```bash
pg_isready -h <host> -p 5432
cd devtrack_server
UV_CACHE_DIR=/tmp/devtrack-uv-cache uv run python -m backend.db.migrate
```

Verify the URL uses `postgresql://`, includes a database name, and is reachable from the server or
container where DevTrack runs.

## Ollama unavailable or slow

```bash
curl "$OLLAMA_HOST/api/tags"
devtrack doctor
```

Confirm `OLLAMA_HOST` includes `http://` or `https://`. Increase `LLM_REQUEST_TIMEOUT_SECS` or
`HTTP_TIMEOUT_LONG` for a healthy but slow local model. DevTrack falls back rather than blocking Git.

## Queue is busy or empty

Use `devtrack queue` or `devtrack queue list`; both are supported. SQLite waits according to
`SQLITE_BUSY_TIMEOUT_MS` during normal daemon concurrency. If no action appears, inspect
`devtrack logs -f` and confirm the branch or commit contains a resolvable ticket ID.

## Commits are not observed

```bash
devtrack status
devtrack workspace list
devtrack is-workspace
devtrack logs -f
```

Confirm the repository is enabled in `workspaces.yaml` and the daemon was restarted or reloaded after
configuration changes.

## Command not found

Run `devtrack help`. Removed commands such as `devtrack health`, `devtrack server-tui`, and
`devtrack admin-start` are not client commands. Use `status`/`doctor`; run server-operator tools from
`devtrack_server/` with `uv run python -m backend.server_tui` or `backend.admin`.

## Getting help

Open a GitHub issue with the DevTrack version, OS, deployment mode, redacted `doctor` output, and the
smallest relevant log excerpt. Never include credentials or private source.
