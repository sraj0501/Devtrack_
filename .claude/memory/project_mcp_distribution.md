---
name: MCP distribution rules
description: Durable local MCP boundary, packaging rules, and unresolved Glama action
type: project
---

**Boundary:** `devtrack mcp` is a local stdio server backed by the Go client's SQLite database. Its tools are read-only, non-destructive, idempotent, and local/no-open-world; the Python HTTP server is not an MCP transport.

**Packaging:** `mcpb/manifest.template.json` is the manifest source. `scripts/package_mcpb.py` packages the matching executable, manifest, README, and license. Packaged launches use `devtrack mcp serve --database PATH` so the user can select the local database explicitly.

**Release rule:** future releases must retain native Windows/macOS/Linux smoke tests, privacy and release-note review, checksum verification, generated `server.json`, and GitHub-OIDC registry publication.

**Unresolved:** record the exact approved Glama listing path and update the score badge on awesome-mcp-servers PR #13608. Copy the path from Glama rather than guessing slug normalization.

Third-party forms, uploads, posts, and external PR changes require explicit authorization and an authenticated owner session.
