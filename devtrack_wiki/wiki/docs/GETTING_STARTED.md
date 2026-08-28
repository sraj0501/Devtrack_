# Getting started

The latest public release is **v3.0.10**. The Phase 9 onboarding, MCP, demo, and runtime-reliability
changes currently live on `dev` and remain unreleased until a newer tag is published.

## Install and configure

Download the correct asset from [GitHub Releases](https://github.com/sraj0501/Devtrack_/releases):

- Linux/macOS: `devtrack_<os>_<arch>.tar.gz`
- Windows: `devtrack_windows_amd64.exe`

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
