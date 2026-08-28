# Verification

## Installed application

```bash
devtrack version
devtrack doctor
devtrack start
devtrack status
devtrack mcp test
devtrack queue
devtrack work report
```

In managed mode, `doctor` must distinguish immediately available Go capabilities from Python,
PostgreSQL, and LLM readiness. A degraded optional capability must not be reported as a broken Git
workflow.

## Source checkout

```bash
cd devtrack_client
GOCACHE=/tmp/devtrack-go-cache GOMODCACHE=/tmp/devtrack-go-mod go test ./...
GOCACHE=/tmp/devtrack-go-cache GOMODCACHE=/tmp/devtrack-go-mod go vet ./...

cd ../devtrack_server
UV_CACHE_DIR=/tmp/devtrack-uv-cache uv run pytest backend/tests/ -q
```

The Python server test environment needs a valid PostgreSQL lane where integration behavior is under
test. Tests that mutate `DATABASE_DIR` or `LLM_PROVIDER` must isolate and reset their state.

## Documentation and website

```bash
python3 devtrack_wiki/check_inline_js.py
sh -n devtrack_wiki/wiki/install.sh
git diff --check
```

Also validate internal wiki page IDs and links before publishing.
