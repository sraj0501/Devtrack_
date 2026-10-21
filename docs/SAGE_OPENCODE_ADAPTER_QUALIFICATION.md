# Sage OpenCode adapter qualification

TASK-166 adds OpenCode as DevTrack Sage's fourth independently managed capture adapter. This is
behavior that was qualified on the development branch and now ships in stable v3.2.0.

## Reference and supported surface

The port was reconciled on 2026-10-15 against the current clean
`C:\Work\git_apps\ai_sessions_skills` HEAD `ea2e067`. That reference installs
`ai-session-skills.js` under OpenCode's user plugin directory and observes both
`tool.execute.before` and `tool.execute.after` because `after` is not reliable in every OpenCode
version.

DevTrack keeps those two tool-event surfaces but adapts away the reference plugin's
`session.idle`/session-process behavior. DevTrack's existing daemon owns Sage lifecycle, liveness,
retry state, and shutdown. The generated JavaScript invokes the packaged Go executable directly
through Bun or Node process APIs; it uses no shell, Python shim, model call, network call, or second
watcher.

```text
devtrack sage harness list
devtrack sage harness install opencode
devtrack sage harness uninstall opencode
```

## Installation contract

- Linux and macOS use `$XDG_CONFIG_HOME/opencode/plugin/ai-session-skills.js`, defaulting to
  `~/.config`. Windows uses `%APPDATA%\opencode\plugin\ai-session-skills.js` unless an existing
  `~/.config/opencode` directory is the established OpenCode home.
- A missing plugin is created atomically and reports `created`; a changed owned/known-legacy plugin
  reports `updated`; an identical reinstall reports `unchanged` without rewriting bytes.
- An unknown file at the dedicated plugin path is never overwritten or removed. Install fails
  closed and uninstall reports `unchanged`, preserving its exact bytes.
- Uninstall removes only the owned or known-legacy dedicated plugin. A missing target reports
  `missing` and creates nothing.
- Executable paths are serialized as JSON strings rather than interpolated into shell commands.
  Node uses `windowsHide: true`, and all child output is ignored.
- Selecting OpenCode never installs or mutates Codex, Claude, or Copilot configuration.

## Capture and privacy contract

The plugin forwards a bounded `PostToolUse`-shaped payload to
`devtrack sage hook opencode --devtrack-sage-hook`. Both plugin callbacks are fire-and-forget,
ignore stdout/stderr, and catch failures so capture cannot break the agent tool loop. Identical
successful before/after observations derive the same fallback identity and collapse during
spool/import deduplication. A failed completion remains distinguishable from its pre-execution
observation, matching the reference's two-event behavior.

The hidden endpoint requires a session identity, admits only an explicit shell, search/glob, or
edit/write tool allowlist, and ignores session events, unknown tools, malformed input, and
oversized input. Raw output, prompts, reasoning, working-directory paths, search values, and edit
contents are never copied. Session and project values become opaque hashes, and every event is
revalidated through the versioned Sage decoder before spooling.

## Verification

The TASK-166 implementation passes:

- focused installer, normalizer, registry, packaged-endpoint, and selective CLI tests;
- creation, reference platform resolution, byte-stable reinstall, owned/legacy removal, unknown
  file preservation, missing uninstall, mode preservation, and independent-adapter checks;
- shell success/failure plus search/edit normalization, privacy canaries, deterministic identity,
  malformed/oversized/session/unknown rejection, and Sage event validation;
- generated-plugin syntax validation with Node 22;
- native Windows executions of the OpenCode hook, registry, packaged endpoint, and selective CLI
  tests;
- the full Go suite, `go vet ./...`, and race tests for the changed Sage packages.

PR #288 targets `dev`, is mergeable, and all 19 hosted checks pass on final head `adca3e7`.
Stable-release claims must wait for a later release qualification and promotion.
