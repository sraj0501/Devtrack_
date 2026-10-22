# CLI quickstart

```bash
devtrack setup
devtrack doctor
devtrack start
devtrack status
```

The setup wizard writes the registered runtime environment immediately. In managed mode, Python,
`uv`, and an optional Ollama model continue bootstrapping in the background.

## Immediate local value

```bash
devtrack mcp setup
devtrack mcp test
devtrack queue list
devtrack logs -f
```

MCP reads local SQLite without Python or Ollama. On v3.3.0 development, queue commands also
query server history; listing can display local rows with an explicit server warning, while
`queue status` errors if server history is unavailable.

## Daily commands

```bash
devtrack work start PROJ-123
devtrack work status
devtrack work report
devtrack eod generate
devtrack queue list
devtrack queue approve <id>
devtrack queue reject <id>
```

Report generation requires the Python server and can stage a new action. Use `devtrack eod show`
to read an existing report. Keep the `server:` prefix when reviewing a server-owned action.

## Diagnosis and recovery

```bash
devtrack status
devtrack doctor
devtrack doctor --repair
devtrack settings
devtrack logs -f
```

Run `devtrack help` for the executable command list. Server-operator tools such as the Python server
TUI and admin UI are run from `devtrack_server/`; they are not Go-client commands.
