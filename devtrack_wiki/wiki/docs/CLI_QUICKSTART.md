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

MCP and queue reads use local SQLite and do not wait for Python or Ollama.

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
