# Usage guide

## Daily workflow

1. Configure a repository with `devtrack setup` or `devtrack workspace add`.
2. Name branches with a ticket ID, for example `feature/PROJ-123-description`.
3. Start the daemon once with `devtrack start` or install autostart.
4. Commit normally. The daemon observes without prompting.
5. Review staged work with `devtrack queue list` or `devtrack tui`.
6. Preview the day with `devtrack work report` or `devtrack eod generate`.

## Signal priority

DevTrack resolves ticket context from a full canonical branch match, an explicit first-line commit
prefix or `Refs:` trailer, or an explicit `devtrack work start`—in that order. Otherwise the commit
is unlinked. Incidental prose IDs, recent tickets, and LLM suggestions are never authoritative.
Conflicts are retained for correction and cannot stage outbound work. None of these states blocks
Git.

```bash
devtrack ticket convention
devtrack ticket check feature/PROJ-123-description
devtrack ticket link abc1234 PROJ-123
```

## Optional interactive Git wrapper

`devtrack git commit` can refine the message before Git runs. After a successful commit it returns
without asking about tickets, time, PM posting, or pushing. This explicitly invoked helper is
separate from the silent daemon path.

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
