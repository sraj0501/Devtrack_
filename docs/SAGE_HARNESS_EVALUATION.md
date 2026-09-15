# SAGE-001 harness evaluation

Checked against published documentation on 2026-09-14. This is a **provisional** contract
comparison, not an installed adapter or a live payload validation. Synthetic v1 fixtures do not
count as observed harness fixtures.

A disposable nested Codex session was launched after the host session restarted, but its shell
tool process was blocked by the enforced read-only sandbox before execution. No `PostToolUse`
event was therefore observed, and the disposable hook configuration was removed. The hook
normalizer and its fixture remain documentation-derived rather than runtime-verified.

| Candidate | Command/result events | Installer considerations | First-slice assessment |
|---|---|---|---|
| Codex | `PostToolUse` covers Bash/unified exec (including nonzero exits), apply-patch, MCP, and most local function tools. The current Python reference also reads stable `commandExecution` items from Codex history for `vscode` and `cli` threads. | User/repo hooks require trust review. On Windows, the reference avoids per-command hooks when its read-only history poller is enabled because the hook host can allocate visible console windows. | **Provisional first choice:** use read-only history import on Windows and retain the hook normalizer for platforms where live fixtures confirm it. The history reader must be feature-flagged and failure-isolated. |
| Claude Code | `PostToolUse` and separate `PostToolUseFailure` provide success/failure signals; both carry tool input and call identity. | Settings hooks receive JSON on stdin; installer must preserve foreign entries and use a bounded silent command handler. | Strong fallback if Codex's observed result shape or trust workflow blocks the first slice. |
| Gemini CLI | `AfterTool` carries original arguments and a response with optional error; base fields include session ID and timestamp. | Command hooks in settings; installer must preserve foreign settings and confirm failure/exit semantics in a live fixture. | Plausible expansion adapter; failure-status mapping needs verification. |

Sources: [official OpenAI Docs: Codex Hooks](https://learn.chatgpt.com/docs/hooks),
[Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference),
[Claude Code hooks guide](https://code.claude.com/docs/en/hooks-guide),
[Claude Agent SDK hook input types](https://code.claude.com/docs/en/agent-sdk/typescript), and
[Gemini CLI hooks reference](https://geminicli.com/docs/hooks/reference/).

Before confirming the SAGE-002 adapter, capture a benign command success and failure event from
the current locally installed harness into a disposable directory, strip IDs, paths, output,
and secrets, and check in only the minimal sanitized shapes. Confirm that `tool_use_id` remains
stable across duplicate deliveries, what `tool_response` says about nonzero shell exits, how
Code-mode nested calls appear, and whether the hook runs on Windows after trust review. Do not
read or commit the user's real transcript or install hooks into their live config for this test.

## Supplemental compatibility review (2026-09-15)

The pinned parity baseline remains `94a2544f8c85a630fa8b5d9a94d9938121aef11b`. Review of the
current `ai_sessions_skills` commit `b85a1ab` found two relevant later changes: configuration now
enables Codex history sources `vscode` and `cli`, and Windows installation omits the per-command
Codex hook when the read-only history poller is active. A synthetic Go contract fixture now covers
that item shape and both allowed sources. The reported Warp compatibility relationship is a
reasonable inference—Warp launches the Codex CLI path—but neither the commit nor code names Warp,
so Sage does not introduce a fake `warp` source.

## Packaged SAGE-002 acceptance (2026-09-15)

`scripts/e2e-sage-capture.ps1` builds a fresh Windows executable and uses disposable data and Codex
homes. It verifies history-mode installation, preservation of an unrelated hook, a silent bounded
hook invocation, privacy-canary exclusion, backlog visibility, and removal. The run passes. The
installer also has Go coverage for repeated install/remove, malformed JSON, UTF-8 BOM input, and
the non-Windows `PostToolUse` merge. This does not replace Codex's trust review: official Codex
documentation says changed non-managed hooks are skipped until reviewed with `/hooks`. A sanitized
event from a user-trusted live hook therefore remains an explicit external validation gate.
