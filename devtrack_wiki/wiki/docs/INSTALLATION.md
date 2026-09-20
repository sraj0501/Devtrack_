# Installation

The canonical maintained installation guide is
[`docs/INSTALLATION.md`](../../../docs/INSTALLATION.md). This page summarizes the supported paths.

## Release assets

Download from [GitHub Releases](https://github.com/sraj0501/Devtrack_/releases):

| Platform | Asset |
|---|---|
| Linux amd64/arm64 | `devtrack_linux_<arch>.tar.gz` |
| macOS amd64/arm64 | `devtrack_darwin_<arch>.tar.gz` |
| Windows amd64 | `devtrack_windows_amd64.exe` |

The release pipeline does not publish a Windows ZIP or a `devtrack-server` release bundle.

## Setup

```bash
devtrack setup
devtrack doctor
devtrack start
devtrack status
```

Choose one deployment mode:

- **Managed:** local Go client plus a background-managed Python server. PostgreSQL is required.
- **Lightweight:** Go-native monitoring, queue, MCP, scheduling, and connectors without Python.
- **External:** connect to an independently operated Python server with `DEVTRACK_SERVER_URL`.

Managed setup returns before sparse checkout, `uv sync --extra ai`, or model downloads finish. The
AI dependency group installs ChromaDB, and local Ollama setup prepares both the selected generation
model and `nomic-embed-text` for first-run voice seeding. Follow progress with `devtrack status`;
retry failures with `devtrack doctor --repair`.

## PostgreSQL

The Python server validates `POSTGRES_URL` and advances Alembic migrations before accepting traffic.
For a local container, use the Compose configuration documented in the canonical installation guide.
The Go client remains SQLite-only and never receives PostgreSQL credentials.

## Source checkout

Contributors build and test with:

```bash
cd devtrack_client && go build -o devtrack .
cd ../devtrack_server && uv sync --extra ai
```

Python dependencies use `uv`, never `pip`.
