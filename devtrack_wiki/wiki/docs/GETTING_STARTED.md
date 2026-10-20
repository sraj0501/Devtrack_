# Getting started

The **v3.2.0 release candidate** includes DevTrack Sage, deterministic ticket mapping, the ten-tool
local MCP server, Phase 9 onboarding, privacy-declaring native MCPB bundles, and the silent
native-Git path. v3.1.1 remains the latest public release until TASK-171 completes.

## Install and configure

Download the correct asset from [GitHub Releases](https://github.com/sraj0501/Devtrack_/releases):

- Linux/macOS: `devtrack_<os>_<arch>.tar.gz`
- Windows: `devtrack_windows_amd64.exe`
- MCP clients: the matching `devtrack_mcpb_<os>_<arch>.mcpb`

Then run:

```bash
devtrack setup
devtrack doctor
devtrack start
devtrack status
```

Managed mode requires a reachable PostgreSQL database for the Python server. The Go client still
starts with its local SQLite capabilities while the managed server and model bootstrap in the
background.

## First value moments

```bash
devtrack mcp setup
devtrack mcp test
devtrack work report
devtrack queue list
```

Use the full canonical branch grammar, such as `feature/PROJ-123-description`. Arbitrary branches
that merely contain a ticket-looking substring do not map. DevTrack observes normal commits,
resolves the ticket deterministically, and stages non-conflicting outbound work with explicit
confidence. Run `devtrack ticket convention` to see the workspace rule.

See [Installation](INSTALLATION.md), [Configuration](CONFIGURATION.md), and
[Troubleshooting](TROUBLESHOOTING.md) for details.
