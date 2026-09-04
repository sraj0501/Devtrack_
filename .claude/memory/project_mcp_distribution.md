---
name: MCP distribution readiness
description: Local MCP boundary, MCPB packaging, protocol metadata, and public-release gates
type: project
---

**Why:** TASK-148 created the reproducible distribution path; TASK-150 qualified and shipped it in v3.1.0.
**Boundary:** `devtrack mcp` is a local stdio server backed by the Go client's SQLite database. Its six tools are read-only, non-destructive, idempotent, and local/no-open-world; the Python HTTP server is not an MCP transport.
**Packaging:** `mcpb/manifest.template.json` uses MCPB manifest 0.3. `scripts/package_mcpb.py` packages the matching executable, manifest, README, and license. Release automation targets Windows amd64, macOS amd64/arm64, and Linux amd64/arm64; CI validates darwin, Linux, and Windows manifest variants with the official MCPB CLI.
**Database:** packaged launches use `devtrack mcp serve --database PATH`, allowing the user to select the local `devtrack.db` file explicitly.
**Release state:** v3.1.1 contains five native MCPBs with `privacy_policies` metadata and a bundled Privacy Policy section. All targets passed native runner execution and clean-project setup. The 10 binary/MCPB payloads match `checksums.txt`, and `io.github.sraj0501/devtrack` 3.1.1 is active/latest in the official MCP Registry with the same five MCPB hashes.
**How to apply:** future releases must retain native Windows/macOS/Linux smoke tests, privacy/release-note review, checksum verification, generated `server.json`, and GitHub-OIDC registry publication. Third-party forms, uploads, posts, and external PRs still require explicit authorization and an authenticated owner session.
