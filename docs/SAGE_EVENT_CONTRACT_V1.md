# DevTrack Sage normalized event contract v1

Status: SAGE-001 foundation. No harness adapter is installed and there is no spool writer,
importer, or search index. Pure Codex hook and history-item normalizers exist in
`internal/sage/hooks/`, but neither is connected to Codex or the daemon. The checked-in v1
fixtures are synthetic contract data, **not** observed harness payloads.

## Local ownership and roots

- The Go client owns Sage. The Python reference is development evidence only.
- Sage's local state root is `<config.DevtrackDataHome()>/sage/`; it honors the client's existing
  `XDG_DATA_HOME` resolution. The current pause marker is `paused` under that root.
- SAGE-002 will create `spool/` beneath this root for immutable, bounded hook events. Only the
  importer will open the existing client SQLite database. The hook will not open SQLite, call a
  model, wait for the daemon, or make network requests.
- No raw event, transcript, command output, or reasoning is sent to PostgreSQL or telemetry.

## JSON facts

`schema_version` is exactly `1`; unknown fields, malformed JSON, trailing data, and inputs over
16 KiB are rejected. Required fields are `event_id`, `harness`, `session_id`, `event_type`
(`command` in v1), `tool`, `occurred_at` (RFC 3339), `command`, and `signature`. Optional fields
are opaque `project_id`, `success`, and `exit_code`. Identifiers are bounded opaque tokens, not
paths. `command` is a safe display form, at most 4096 bytes; `signature` is a bounded lowercase
action key. The adapter must omit raw command output and reasoning entirely.

The parser rejects obvious credential markers and home-directory paths, and returns errors that
never echo input. This is a guardrail, **not** proof that arbitrary raw commands are safe. Adapters
must normalize and minimize before calling the parser; until that behavior passes privacy tests,
they must drop ambiguous commands rather than persist them. `DeliveryKey()` hashes harness,
session, and event ID so the SAGE-002 importer can make repeated deliveries idempotent. The key
alone does not deduplicate a database yet.

The provisional Codex normalizer currently accepts only `PostToolUse`/`Bash` with a bounded
128 KiB payload and a small allowlist of simple Git, Go, npm, and uv actions. It derives the
display command from the action and selected safe flags; free-text arguments, raw output,
transcript paths, and cwd are never copied. Session, call, and project identifiers are hashed.
It deliberately leaves `success` and `exit_code` unknown because the published Codex hook
contract describes `tool_response` as tool-specific, not a stable exit-status schema. It drops
unsupported or ambiguous payloads silently. This is an initial privacy seam, not full command
coverage.

The supplemental Windows compatibility seam accepts completed or failed Codex
`commandExecution` history items only when their thread source is `vscode` or `cli`. It reads the
original command from `commandActions`, preserves a consistent exit status, deduplicates repeated
actions within an item, and applies the same command minimization. Output, cwd, thread IDs, and
item IDs are never copied. This shape is derived from the Python reference compatibility fix at
`b85a1ab`; the Go code does not yet open Codex SQLite databases. `cli` covers terminal-launched
Codex sessions generally. The repository contains no Warp-specific branch or identifier.

## CLI compatibility and exit behavior

`devtrack sage status`, `pause`, `resume`, and `doctor` return a single JSON state object on
stdout. `capture: "not_installed"` and `ready: false` explicitly distinguish this foundation from
a working capture pipeline. Pause/resume are idempotent and persist locally. Success exits 0;
invalid arguments and state I/O errors exit nonzero. The old `sage ask|do|pr|interactive` forms
continue to work and emit a migration notice on stderr. `sage git ask|do|pr|interactive` are
the explicit aliases. `search`, `topics`, and `install-hooks` currently fail clearly rather than
silently launching the old Git agent. Their actual implementations belong to SAGE-002/003.

## Open SAGE-001 work

Validate current official hook contracts and collect sanitized real fixtures for candidate
harnesses. Score event completeness, installer safety, and daily usage before selecting the first
SAGE-002 adapter. The provisional comparison is in
[SAGE_HARNESS_EVALUATION.md](SAGE_HARNESS_EVALUATION.md). The Python scenario assignment is in
[SAGE_PORT_PARITY_MATRIX.md](SAGE_PORT_PARITY_MATRIX.md).
