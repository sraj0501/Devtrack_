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
- Reports completed work without tracking hours or time spent per task in the v3.3.0 development line.
- Maps ticket-named branches such as `feature/AUTH-42-refresh-token` to work items.
- Builds reviewable ticket updates and end-of-day reports from the work already completed.
- Keeps its client-side source of truth in local SQLite and uses Ollama by default for local AI.
- Shares current work and DevTrack Sage knowledge with compatible assistants through ten read-only
  MCP tools.

Nothing is posted merely because DevTrack generated it. PM updates, transitions, and deliveries are
placed in a pending-actions queue before delivery; configured earned auto-approval rules can advance
eligible actions. Anonymous telemetry is also opt-in and disabled by
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

Stable v3.2.0 retains the former time-tracking behavior. The fixes and removal are on `dev` at
`860b91b`, prepared for v3.3.0; rolling development builds use `dev-<commit>`. A push to `dev` does
not upgrade an installed binary or publish a new stable release. See the
[development release notes](docs/releases/v3.3.0.md) and [qualification plan](docs/testing/basic-workflow.md).

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

Managed mode starts the local Python server for AI-written ticket comments, EOD narratives,
Boardroom reviews, planning, and voice personalization. External mode does not start Python: point
`DEVTRACK_SERVER_URL` at a separately operated Python/PostgreSQL server for those features. With no
server reachable, commit capture, deterministic ticket mapping, local history, queue, and MCP still
work; generated reports and other server-backed features are unavailable. Model failures on a
reachable server fall back to raw or templated wording. External mode skips automatic first-run
voice mining. See the [LLM guide](devtrack_wiki/wiki/docs/LLM_GUIDE.md).

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

In the development line, capture enumerates unseen commits across local branch tips, so rapid
commits and switching branches during processing are included. Keep the daemon running: existing
history is seeded at startup, and catch-up for commits made while it was stopped is not guaranteed.

`devtrack work start AUTH-42` selects fallback ticket context; `work stop` clears it and `work status`
shows it. These commands do not start or stop timers. `devtrack work report` generates a commit-based
EOD report through the configured server.

Incidents, stories, bugs, and tasks all map by their work-item reference: for example, a Jira
incident `hotfix/OPS-123-restore-login`, Azure DevOps story `feature/ADO-456-add-export`, or
GitHub issue `fix/GH-42-handle-timeout`. The PM system supplies the item type; the branch kind
does not. A canonical branch wins over an explicit commit reference, which wins over selected
ticket context. Run `devtrack ticket convention` and `devtrack ticket check <branch>` to inspect
the rule, or `devtrack ticket link <commit-hash> <ticket-ref>` to correct an observed mapping.
See [work-item and ticket mapping](docs/TICKET_MAPPING.md) for configuration, conflicts, and examples.

`devtrack eod status` and `devtrack eod show` read the latest report across local and server history.
`devtrack queue list --all` shows recent actions from both stores. Server actions use IDs such as
`server:123`: pass that full ID to `queue approve`, `queue reject`, or `queue edit`. Bare numeric IDs
address local actions. An unavailable server is reported explicitly.

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
| Qualify automatic capture and reporting | [Basic workflow testing](docs/testing/basic-workflow.md) |
| Map incidents, stories, bugs, and tasks | [Work-item and ticket mapping](docs/TICKET_MAPPING.md) |
| Review the v3.3.0 development boundary | [Development release notes](docs/releases/v3.3.0.md) |

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
