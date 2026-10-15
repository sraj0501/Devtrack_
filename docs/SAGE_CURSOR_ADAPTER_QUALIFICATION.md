# Sage Cursor adapter qualification

TASK-167 adds Cursor as DevTrack Sage's fifth independently managed capture adapter. This is
development-branch behavior and is not part of stable v3.1.1.

## Reference and supported surface

The port was reconciled on 2026-10-15 against the current clean
`C:\Work\git_apps\ai_sessions_skills` HEAD `ea2e067`. The reference merges a version-1
`hooks.json` under Cursor's home-relative `.cursor` directory and observes
`afterShellExecution` plus `postToolUse` matched to `Shell|Write`.

DevTrack keeps those capture surfaces but adapts away the reference `sessionStart` and
`sessionEnd` hooks. The existing DevTrack daemon owns Sage lifecycle, liveness, retry state, and
shutdown; installing Cursor does not add a watcher, session process, Python shim, model call, or
network dependency.

```text
devtrack sage harness list
devtrack sage harness install cursor
devtrack sage harness uninstall cursor
```

## Installation contract

- Every supported platform resolves Cursor's hook file to `~/.cursor/hooks.json`; an explicit
  installer home remains available for isolated tests.
- A missing file is created atomically with version 1 and reports `created`. Reinstalling identical
  owned entries reports `unchanged` and preserves exact bytes.
- Install merges one owned `afterShellExecution` entry and one owned `postToolUse` entry with the
  `Shell|Write` matcher. Both invoke the packaged DevTrack executable and carry a ten-second host
  timeout.
- Existing settings, foreign hook entries, empty foreign objects, and file mode survive install,
  reinstall, and uninstall. Known legacy `ai_sessions_skills` entries are removed before current
  entries are added.
- Uninstall removes only owned or known-legacy entries. A missing hook file reports `missing` and
  creates nothing.
- Selecting Cursor never installs or mutates Codex, Claude, Copilot, or OpenCode configuration.

## Capture and privacy contract

The hidden endpoint accepts bounded `afterShellExecution` and `postToolUse` JSON. Top-level Cursor
shell commands and matched `Shell` or `Write` tool payloads are normalized through the shared Sage
capture layer. Unknown tools, lifecycle events, malformed JSON, oversized input, and incomplete
shell payloads are ignored silently.

Cursor shell payloads can omit a session identifier. When that happens, DevTrack derives an opaque
deterministic local identity from the harness and available project context. Supplied session,
conversation, generation, call, and project values are hashed; raw paths and identifiers are not
persisted. Equivalent successful shell observations from `afterShellExecution` and `postToolUse`
produce the same delivery key and collapse in the spool/import path.

Raw output, prompts, reasoning, working-directory paths, file paths, and written contents are never
copied. Each normalized event is revalidated through the versioned Sage decoder before spooling.

## Verification

The TASK-167 implementation passes:

- focused installer, normalizer, registry, packaged-endpoint, and selective CLI tests;
- additive merge, byte-stable reinstall, owned/legacy removal, invalid-file preservation, missing
  uninstall, mode preservation, home-relative path, and independent-adapter checks;
- reference-shaped shell success/failure and write normalization, privacy canaries, cross-hook
  deterministic identity, malformed/oversized/lifecycle/unknown rejection, and Sage validation;
- native Windows executions of the Cursor hook, registry, packaged endpoint, and selective CLI
  tests;
- the full Go suite, `go vet ./...`, and race tests for the changed Sage packages.

Hosted PR checks remain the final integration gate. Stable-release claims must wait for a later
release qualification and promotion.
