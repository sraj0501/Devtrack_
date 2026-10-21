# Sage GitHub Copilot CLI adapter qualification

TASK-165 adds GitHub Copilot CLI as DevTrack Sage's third independently managed capture adapter.
It was qualified as development-branch behavior and now ships in stable v3.2.0.

## Reference and supported surface

The port was reconciled on 2026-10-15 against the current clean
`C:\Work\git_apps\ai_sessions_skills` HEAD `ea2e067`. That reference writes a version-1 Copilot
hook document to `~/.copilot/hooks/ai-session-skills.json`, supplies both `bash` and `powershell`
commands, sets `AI_SKILLS_HARNESS=copilot`, and uses `timeoutSec: 10`.

DevTrack intentionally installs only one owned `postToolUse` entry. The reference's
`sessionStart` and `sessionEnd` processes are adapted away because DevTrack's existing daemon owns
Sage lifecycle, liveness, retry state, and shutdown. The packaged Go executable is invoked
directly; no Python shim or second watcher is installed.

```text
devtrack sage harness list
devtrack sage harness install copilot
devtrack sage harness uninstall copilot
```

## Installation contract

- Install creates the dedicated version-1 hook file when missing and reports `created`; a later
  content change reports `updated`, and an identical reinstall reports `unchanged` without
  rewriting bytes.
- Existing document keys, other hook groups, foreign `postToolUse` entries, and the existing file
  mode survive install and uninstall.
- Only entries carrying DevTrack's owner marker or a known legacy `ai_sessions_skills` marker are
  replaced or removed. Uninstalling a missing adapter reports `missing` and creates nothing.
- A non-version-1 document, a non-object `hooks` value, or a non-array `postToolUse` value is
  rejected without overwriting the user's file.
- Executable paths are safely quoted for POSIX shells and PowerShell, including spaces and single
  quotes. The entry carries `AI_SKILLS_HARNESS=copilot` and `timeoutSec: 10`.
- Selecting Copilot never installs or mutates the Codex or Claude adapters.

## Capture and privacy contract

The hidden `devtrack sage hook copilot --devtrack-sage-hook` endpoint reads one bounded JSON
payload from standard input and always remains silent and fail-open. It accepts `postToolUse`
payloads for an explicit shell, search/glob, or edit/write allowlist and ignores session events,
unknown tools, malformed input, missing session identity, and oversized input.

The normalizer reuses Sage's shared command minimizer. It never copies raw tool results, command
output, prompts, reasoning, working-directory paths, search values, or edit contents. Session and
project values become opaque hashes; normalized events are revalidated through the versioned Sage
event decoder before spooling.

Copilot payloads do not guarantee a tool-call identifier. DevTrack prefers `toolCallId`,
`toolUseId`, `tool_use_id`, `callId`, or `timestamp`; when none exists, it derives a deterministic
identity from the session, tool, minimized signature, and success state. This intentionally favors
duplicate suppression over counting two indistinguishable calls twice.

## Verification

The following passed for the TASK-165 implementation:

- focused Linux tests for `./internal/sage/hooks`, `./internal/sage/harness`, and the client CLI;
- installer creation, additive preservation, byte-stable reinstall, pure subtraction, malformed
  configuration, legacy replacement, portable quoting, and truthful status/outcome tests;
- shell success/failure plus search/edit normalization, privacy canaries, deterministic identity,
  unsupported/session rejection, malformed input, and size-bound tests;
- native Windows executions of the Copilot hook, registry, packaged endpoint, and selective CLI
  tests;
- the full Go suite, `go vet ./...`, and race tests for the changed Sage packages.

PR #287 targets `dev`, is mergeable, and all 19 hosted checks pass on reviewed head `197428e`.
Stable-release claims must wait for a later release qualification and promotion.
