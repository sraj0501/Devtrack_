# Installation

The maintained development installation details live in
[`docs/INSTALLATION.md`](https://github.com/sraj0501/Devtrack_/blob/dev/docs/INSTALLATION.md).
This page gets a new user to a version check; the [Quick Start](QUICK_START.md) then configures
the first repository. Git is required on the machine that runs DevTrack.

## Release assets

Download from [GitHub Releases](https://github.com/sraj0501/Devtrack_/releases):

| Platform | Asset |
|---|---|
| Linux x86-64 (`amd64`) / ARM64 | `devtrack_linux_amd64.tar.gz` / `devtrack_linux_arm64.tar.gz` |
| macOS Intel (`amd64`) / Apple Silicon (`arm64`) | `devtrack_darwin_amd64.tar.gz` / `devtrack_darwin_arm64.tar.gz` |
| Windows amd64 | `devtrack_windows_amd64.exe` |

The release pipeline does not publish a Windows ZIP or a `devtrack-server` release bundle.
Download and extract the matching archive, then put the `devtrack` binary on your `PATH`.
On Windows, put the `.exe` in a directory on `PATH`; you may rename it `devtrack.exe`.
Run `devtrack version` to confirm which build you actually installed. The latest public release
is v3.2.0; v3.3.0 changes described in development pages require a `dev-<commit>` build.

## Setup

From inside the Git repository you want to monitor:

```sh
devtrack setup
devtrack doctor
devtrack start
devtrack status
```

Choose one deployment mode:

- **Managed:** local Go client plus a background-managed Python server. PostgreSQL is required.
- **Local-only evaluation:** choose External in the wizard and leave the server URL unset. The client still tries its loopback default; with no Python server listening, Go-native monitoring, queue, and MCP remain available. The `lightweight` environment value also disables managed server startup, but is not a wizard menu choice.
- **External:** connect to an independently operated Python server with `DEVTRACK_SERVER_URL`.

For an initial local evaluation, select **External** and **None** as the PM platform. It needs
neither the Python server nor PM credentials. Select Managed when you want generated reports.
Managed setup returns before sparse checkout, `uv sync --extra ai`, or model downloads finish. The
AI dependency group installs ChromaDB, and local Ollama setup prepares both the selected generation
model and `nomic-embed-text` for first-run voice seeding. Follow progress with `devtrack status`;
retry failures with `devtrack doctor --repair`.

Continue with [Quick Start](QUICK_START.md) to make and inspect your first mapped commit.

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
