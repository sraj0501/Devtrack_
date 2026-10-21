<div align="center">

# DevTrack

**Never write a standup again.**

DevTrack watches your Git work, connects it to tickets, and prepares status updates and end-of-day
reports in the background. It is local-first, offline-capable, and stages outbound actions for your
review.

[![GitHub Release](https://img.shields.io/github/v/release/sraj0501/Devtrack_?label=release&color=blue)](https://github.com/sraj0501/Devtrack_/releases/latest)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue)](https://devtrack.cloud/download.html)
[![License](https://img.shields.io/badge/license-Community-green)](TERMS.md)

[Website](https://devtrack.cloud) · [Download](https://devtrack.cloud/download.html) · [Documentation](https://devtrack.cloud/wiki.html) · [Latest release](https://github.com/sraj0501/Devtrack_/releases/latest)

![DevTrack demo](devtrack_wiki/wiki/assets/demo.gif)

</div>

## What DevTrack does

- Observes commits without interrupting or replacing normal Git commands.
- Current source reports completed work without tracking hours or time spent per task; this removal is not yet in the v3.2.0 release.
- Maps ticket-named branches such as `feature/AUTH-42-refresh-token` to work items.
- Builds reviewable ticket updates and end-of-day reports from the work already completed.
- Keeps its client-side source of truth in local SQLite and uses Ollama by default for local AI.
- Shares current work and DevTrack Sage knowledge with compatible assistants through ten read-only
  MCP tools.

Nothing is posted merely because DevTrack generated it. PM updates, transitions, and deliveries are
placed in a pending-actions queue until approved. Anonymous telemetry is also opt-in and disabled by
default.

## Get started

DevTrack supports Windows, macOS, and Linux. Git is required. The core Go client, SQLite storage,
Git observation, queue, and MCP features do not require Python or an AI model. Managed AI features
add `uv`, PostgreSQL, and an LLM provider; Ollama is the default.

### 1. Install

Use the guided [download page](https://devtrack.cloud/download.html), or install the latest stable
release directly:

**Linux or macOS**

```sh
curl -fsSL https://devtrack.cloud/install.sh | sh
```

**Windows PowerShell**

```powershell
irm https://devtrack.cloud/install.ps1 | iex
```

Manual binaries and checksums are available on [GitHub Releases](https://github.com/sraj0501/Devtrack_/releases/latest).

```sh
devtrack version
```

### 2. Configure a repository

Run setup from the Git repository you want DevTrack to observe:

```sh
cd /path/to/your/repository
devtrack setup
devtrack start
devtrack status
devtrack doctor
```

The setup wizard configures the operating mode, repository, LLM, identity, and optional PM
integration. Choose `none` for the PM integration when evaluating the local-only path. Managed mode
requires PostgreSQL for the optional Python service; Go-native features remain available while that
service and the model are being prepared.

### 3. Try the workflow

Use a ticket-named branch and commit normally:

```sh
git switch -c feature/AUTH-42-refresh-token
git commit -m "fix auth redirect"

devtrack logs
devtrack queue
devtrack eod
```

`queue` lets you inspect staged actions before approving anything. See the
[Quick Start](https://devtrack.cloud/wiki.html#QUICK_START) for the complete first-run walkthrough.

## Agent memory and DevTrack Sage

Add DevTrack's read-only MCP server to the current project:

```sh
devtrack mcp setup
devtrack mcp test
devtrack mcp status
```

Reload your MCP-capable assistant after setup. It can then read current ticket context, commits,
pending actions, EOD context, voice information, and published Sage knowledge. MCP cannot approve
actions or modify DevTrack data.

DevTrack Sage captures useful command activity from supported coding harnesses and turns repeated
work into searchable local Markdown knowledge:

```sh
devtrack sage status
devtrack sage search "docker compose"
devtrack sage topics
```

Learn more in [MCP and Agent Memory](https://devtrack.cloud/wiki.html#MCP_CLAUDE_CODE) and
[DevTrack Sage](https://devtrack.cloud/wiki.html#SAGE).

## Where to go next

| I want to… | Guide |
|---|---|
| Install or upgrade DevTrack | [Download](https://devtrack.cloud/download.html) |
| Complete the first-run workflow | [Quick Start](https://devtrack.cloud/wiki.html#QUICK_START) |
| Understand deployment choices | [Installation](https://devtrack.cloud/wiki.html#INSTALLATION) |
| Configure repositories, integrations, or models | [Configuration](https://devtrack.cloud/wiki.html#CONFIGURATION) |
| Find a command | [CLI Reference](https://devtrack.cloud/wiki.html#CLI_REFERENCE) |
| Connect an AI assistant | [MCP and Agent Memory](https://devtrack.cloud/wiki.html#MCP_CLAUDE_CODE) |
| Use local command knowledge | [DevTrack Sage](https://devtrack.cloud/wiki.html#SAGE) |
| Fix an installation or runtime problem | [Troubleshooting](https://devtrack.cloud/wiki.html#TROUBLESHOOTING) |
| Review privacy and data flow | [Privacy](https://devtrack.cloud/privacy.html) |

Contributors and maintainers can start with the repository [documentation map](docs/README.md) and
[architecture](docs/ARCHITECTURE.md).

## Privacy

The default Go, SQLite, and Ollama path stays local. External PM systems, delivery channels, and
cloud LLMs receive data only when configured for the corresponding operation. Telemetry remains off
until explicitly enabled with `devtrack telemetry on`. Read the full
[privacy guide](https://devtrack.cloud/privacy.html) before enabling external services.

## License

DevTrack uses the [DevTrack Community License](TERMS.md): free for personal use and teams of up to
10 users; larger teams and hosted use require an enterprise license.
