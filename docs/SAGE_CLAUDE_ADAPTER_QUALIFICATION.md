# Sage Claude Code adapter qualification

TASK-164 adds Claude Code as the second independently managed DevTrack Sage capture adapter. The
implementation follows the Claude hook shape in the authoritative `ai_sessions_skills` checkout at
commit `ea2e067` and was verified against the current Claude Code hook contract on 2026-10-15.

## User surface

```text
devtrack sage harness list
devtrack sage harness install claude
devtrack sage harness uninstall claude
```

Installation updates `~/.claude/settings.json` with DevTrack-owned command hooks for:

- `PostToolUse`: `Bash|Grep|Glob|Edit|MultiEdit|Write|NotebookEdit`
- `PostToolUseFailure`: `Bash`

The hook command calls the packaged Go executable directly. It does not install Python, start a
second watcher, or add Claude session-start/session-end processes; the existing DevTrack daemon
remains the only background owner.

## Safety contract

- Install is additive, preserves unrelated JSON and the existing file mode, and safely quotes an
  executable path containing spaces.
- Reinstall is byte-stable. Uninstall removes only DevTrack-owned or known legacy hook entries and
  reports a missing settings file without creating one.
- Structurally invalid settings fail closed without being overwritten.
- Hook delivery is silent, bounded to 128 KiB, model-free, network-free, and fail-open.
- Normalized events contain opaque session/call/project identifiers and reusable command facts.
  Raw tool output, prompts, reasoning, paths, search text, edit text, and private argument values
  are not copied into the Sage event.
- Unknown events and tools are ignored. Failed-event capture is supported only for `Bash`, matching
  the installed hook contract.

## Qualification evidence

Focused installer, normalizer, registry, and CLI tests pass on Linux and as native Windows test
binaries. The full Go suite, `go vet ./...`, and race coverage for the modified packages pass under
Go 1.24. Privacy-canary tests verify that Claude payload values and raw output do not reach the
spool.

This adapter is part of rolling `dev` development and is not included in stable v3.1.1.
