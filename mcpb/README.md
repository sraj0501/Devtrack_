# DevTrack MCP bundle

This bundle is the local, read-only MCP surface of DevTrack. During installation, select the
`devtrack.db` created by `devtrack setup`. Typical locations are shown by `devtrack status`; the
exact path follows the configured `DATABASE_DIR` and `DATABASE_FILE_NAME` values.

The six tools read active work context, today's commits, pending actions, voice information, ticket
context, and a template EOD summary. Tool calls do not post comments, change tickets, send reports,
or contact the optional Python server.

The bundled executable is governed by the DevTrack Community License included in the archive.
Support: https://github.com/sraj0501/Devtrack_/issues
