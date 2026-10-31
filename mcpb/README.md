# DevTrack MCP bundle

This bundle is the local, read-only MCP surface of DevTrack. During installation, select the
`devtrack.db` created by `devtrack setup`. Typical locations are shown by `devtrack status`; the
exact path follows the configured `DATABASE_DIR` and `DATABASE_FILE_NAME` values.

Ten tools read active work context, today's commits, pending actions, voice information, ticket
context, a template EOD summary, and local Sage knowledge/status. Sage retrieval reconciles the
current deterministic Markdown knowledge directory; it never returns raw capture events or logs.
Tool calls do not post comments, change tickets, send reports, or contact the optional Python
server.

The bundled executable is governed by the DevTrack Community License included in the archive.
Support: https://github.com/sraj0501/Devtrack_/issues

## Privacy Policy

The bundle reads the local SQLite database selected during installation and, for Sage tools, the
local deterministic Markdown knowledge directory. It does not expose raw capture events, logs,
prompts, output, reasoning, or adapter configuration paths. Its MCP tools do not contact the
optional Python server or any hosted MCP endpoint. The DevTrack privacy policy explains local
storage, optional integrations, sharing, retention, deletion, and how to report a privacy or
security concern: https://mogrov.com/wiki/devtrack/privacy.html
