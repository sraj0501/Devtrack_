# Sage Devin CLI adapter qualification

TASK-168 adds Devin CLI as DevTrack Sage's sixth independently managed capture adapter. This is
development-branch behavior and is not part of stable v3.1.1.

## Reference and supported surface

The port was reconciled on 2026-10-16 against the current clean
`C:\Work\git_apps\ai_sessions_skills` HEAD `ea2e067`. The reference merges a `PostToolUse` hook
matched to `^(exec|grep|glob|edit|write|apply_patch)$` into Devin's platform-specific user
configuration file.

DevTrack keeps that capture surface but adapts away the reference `SessionStart` and `SessionEnd`
hooks. The existing DevTrack daemon owns Sage lifecycle, liveness, retry state, and shutdown;
installing Devin adds no watcher, session process, Python shim, model call, or network dependency.

```text
devtrack sage harness list
devtrack sage harness install devin
devtrack sage harness uninstall devin
```

## Installation contract

- Windows resolves `%APPDATA%\devin\config.json`, with the platform home fallback when `APPDATA`
  is absent. Linux and macOS resolve `$XDG_CONFIG_HOME/devin/config.json`, defaulting to
  `~/.config/devin/config.json`.
- A missing file is created atomically and reports `created`. Reinstalling the same owned entry
  reports `unchanged` and preserves exact bytes.
- Install merges one owned `PostToolUse` entry with the reference matcher, ten-second timeout, and
  packaged DevTrack executable. It never adds session lifecycle hooks.
- Existing settings, foreign hook entries, empty foreign objects, and POSIX file mode survive
  install, reinstall, and uninstall. Known legacy `ai_sessions_skills` entries are replaced.
- Uninstall removes only owned or known-legacy entries. A missing configuration reports `missing`
  and creates nothing.
- Selecting Devin never installs or mutates Codex, Claude, Copilot, Cursor, or OpenCode
  configuration.

## Capture and privacy contract

The hidden endpoint accepts bounded Devin `PostToolUse` JSON for `exec`, `grep`, `glob`, `edit`,
`write`, and `apply_patch`. Payloads without an event name remain compatible with the reference
hook shape. Other events, unknown tools, malformed JSON, oversized input, and incomplete shell
payloads are ignored silently.

When Devin omits a session identifier, DevTrack derives an opaque deterministic local identity
from the harness and available project context. Supplied session, call, and project values are
hashed. Tool output, prompts, reasoning, working-directory paths, file paths, patterns, patch text,
and written contents are never copied. Each normalized event is revalidated through the versioned
Sage decoder before spooling.

## Verification

The TASK-168 implementation passes:

- focused installer, normalizer, registry, packaged-endpoint, and selective CLI tests;
- additive merge, byte-stable reinstall, owned/legacy removal, invalid-file preservation, missing
  uninstall, POSIX mode preservation, platform-root path, and independent-adapter checks;
- reference-shaped shell, search, glob, edit, write, and patch normalization, privacy canaries,
  deterministic identity, and malformed/oversized/lifecycle/unknown rejection;
- native Windows executions of the Devin hook, registry, packaged endpoint, and selective CLI
  tests;
- the full Go suite, `go vet ./...`, and race tests for the changed Sage packages.

Hosted review remains the integration gate. Stable-release claims must wait for a later release
qualification and promotion.
