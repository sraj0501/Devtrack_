---
name: MCP distribution readiness
description: Local MCP boundary, MCPB packaging, protocol metadata, and public-release gates
type: project
---

**Why:** TASK-148 turned the existing source-only MCP server into a reproducible distribution path without claiming that unreleased bundles are publicly installable.
**Boundary:** `devtrack mcp` is a local stdio server backed by the Go client's SQLite database. Its six tools are read-only, non-destructive, idempotent, and local/no-open-world; the Python HTTP server is not an MCP transport.
**Packaging:** `mcpb/manifest.template.json` uses MCPB manifest 0.3. `scripts/package_mcpb.py` packages the matching executable, manifest, README, and license. Release automation targets Windows amd64, macOS amd64/arm64, and Linux amd64/arm64; CI validates darwin, Linux, and Windows manifest variants with the official MCPB CLI.
**Database:** packaged launches use `devtrack mcp serve --database PATH`, allowing the user to select the local `devtrack.db` file explicitly.
**Release state:** v3.0.10 predates MCP and includes no MCPB. Source readiness at `c1329f7` is not a public release, client-install result, registry listing, or security review.
**How to apply:** before tagging, require green CI plus install/smoke tests on Windows, macOS, and Linux and privacy-policy/release-note review. After a real release exists, record bundle SHA-256 values, generate and validate `server.json`, and obtain explicit approval before any registry publish, form submission, upload, or external PR.
