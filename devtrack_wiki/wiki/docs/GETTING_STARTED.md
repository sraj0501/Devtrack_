# Getting started

The latest public release is **v3.1.1**. It includes the Phase 9 onboarding, local MCP server,
privacy-declaring native MCPB bundles, demo path, and runtime-reliability work.

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

Use a branch containing a ticket ID, such as `feature/PROJ-123-description`. DevTrack observes
normal commits, resolves the ticket, and stages outbound work with explicit confidence.

See [Installation](INSTALLATION.md), [Configuration](CONFIGURATION.md), and
[Troubleshooting](TROUBLESHOOTING.md) for details.
