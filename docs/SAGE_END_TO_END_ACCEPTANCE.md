# Sage end-to-end acceptance qualification

_Qualified 2026-10-15 against `ai_sessions_skills` commit `ea2e067`._

## Result

The SAGE-003 capture-to-self-written-knowledge journey passes twice from clean, isolated state on
native Windows and twice in Linux Go 1.24. The test is
`internal/infra/sage_acceptance_test.go#TestSageCaptureToKnowledgeAcceptanceTwice`; the existing
Windows/Linux client CI matrix runs the same gate on every relevant pull request.

Each pass proves:

- additive Codex installation and safe uninstall preserve a user-owned hook;
- a realistic PostToolUse payload is silently normalized and spooled without its response, cwd,
  private path, or privacy canary;
- one event is imported exactly once into the durable SQLite event and job queues;
- an unavailable model and malformed model output remain retryable before a valid third response;
- the daemon publishes deterministic Markdown, search returns the complete entry, and restart
  neither changes bytes nor repeats model work;
- a remembered route survives, merge/refile moves the entry, and search plus effective routing
  reflect the corrected destination;
- the optional local commit contains only Sage-owned knowledge while unrelated staged and
  unstaged work remains intact; and
- runtime and knowledge trees contain none of the supplied privacy canaries.

## Validation

- Linux Go 1.24: both clean passes succeeded.
- Native Windows amd64 test binary: both clean passes succeeded.
- Full Linux Go suite and `go vet ./...`: pass.
- Focused Linux race run for the repeated acceptance gate: pass.
- Current-reference parity: 136 scenarios, 119 implemented, 17 adapted, 0 partial, 0 pending.

TASK-157 remains open only for hosted review and integration of the stacked Sage pull requests into
`dev`; no SAGE-003 behavior or acceptance gate remains unimplemented.
PR #285 carries this final acceptance slice; hosted validation is in progress. Parent parity PR
#284 is fully green with all 19 checks passing.
