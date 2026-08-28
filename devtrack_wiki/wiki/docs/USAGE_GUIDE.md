# Usage guide

## Daily workflow

1. Configure a repository with `devtrack setup` or `devtrack workspace add`.
2. Name branches with a ticket ID, for example `feature/PROJ-123-description`.
3. Start the daemon once with `devtrack start` or install autostart.
4. Commit normally. The daemon observes without prompting.
5. Review staged work with `devtrack queue list` or `devtrack tui`.
6. Preview the day with `devtrack work report` or `devtrack eod generate`.

## Signal priority

DevTrack resolves ticket context from the branch name, commit prefix, Git trailer, an explicit
`devtrack work start`, or recent ticket context—in that order. An unlinked commit is logged and never
blocks Git.

## Optional interactive Git wrapper

`devtrack git commit` can refine the message, offer the Go-native ticket picker, record time, and
offer a push. This is an explicitly invoked workflow and is separate from the silent daemon path.

## Pending actions

```bash
devtrack queue list
devtrack queue approve <id>
devtrack queue edit <id> '<json>'
devtrack queue reject <id>
devtrack queue flag <id> "correction"
```

Every external PM, email, or Git action must pass through this queue and carry confidence.

## Work and reports

```bash
devtrack work start PROJ-123
devtrack work status
devtrack work adjust 15
devtrack work stop
devtrack work report
devtrack eod generate
devtrack eod show
```

## Agent context

```bash
devtrack mcp setup
devtrack mcp test
```

MCP reads local SQLite and does not require the Python server.

## Operations

```bash
devtrack status
devtrack doctor
devtrack logs -f
devtrack pause
devtrack resume
devtrack autostart-install
```

Run `devtrack help` for the current command surface.
