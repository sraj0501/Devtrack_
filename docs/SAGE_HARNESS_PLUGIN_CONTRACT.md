# DevTrack Sage harness plugin boundary

Status: architectural contract for post-SAGE-002 adapter expansion.

## User-facing model

Harness capture is opt-in and independently managed:

```text
devtrack sage harness list
devtrack sage harness install codex
devtrack sage harness uninstall codex
```

Installing one adapter must not install, enable, or edit configuration for another harness. The
legacy `install-hooks --harness codex` form remains a compatibility alias.

## In-process registry

Every built-in adapter implements the Go `harness.Adapter` interface: descriptor, status, install,
and uninstall. Payload normalizers remain separately testable pure functions. Codex is the first
registered adapter; Claude Code, OpenCode, Gemini CLI, Cursor, Copilot, Pi, and Hermes can be added
without changing Sage's spool, database, importer, or search layers.

## External extension direction

Dynamic Go plugins are not an extension mechanism because Go's plugin runtime is not portable to
Windows. External adapter packages should use a versioned manifest containing identity,
capabilities, supported platforms, hook locations, and a declarative payload mapping. Packages
must be explicitly installed and trust-reviewed. Declarative adapters may emit only the normalized
Sage event contract; they cannot request network access, model calls, raw transcript persistence,
or arbitrary database access. A future sandboxed WASI normalizer may cover payloads that cannot be
expressed declaratively.

MCP is the management and discovery surface, not the capture transport. Future read-only MCP tools
may list installed/available adapters and diagnostics; an approval-gated tool may request adapter
installation. Hook execution remains local, bounded, silent, and independent of an MCP server's
availability or latency.

## Trust and compatibility rules

- Preserve unrelated harness configuration byte-for-byte where structurally possible.
- Mark every owned entry so repeated install/remove is deterministic.
- Never enable more than the adapter explicitly selected by the user.
- Validate manifests and payloads before filesystem mutation.
- Fail open for hook delivery but fail closed for installation/configuration corruption.
- Keep raw payloads, output, reasoning, secrets, and absolute paths out of normalized events.
