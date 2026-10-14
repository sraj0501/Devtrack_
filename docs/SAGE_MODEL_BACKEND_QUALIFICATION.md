# Sage model backend qualification

Date: 2026-10-14  
Task: TASK-157 / SAGE-003  
Branch: `features/TASK-157-sage-model-backends`  
Reference: `C:\Work\git_apps\ai_sessions_skills` at `ea2e067`

## Delivered behavior

- Ollama HTTP remains the default Sage model transport and does not require the `ollama` executable
  when a compatible endpoint is reachable.
- `DEVTRACK_SAGE_MODEL_BACKEND` can explicitly select `claude`, `devin`, `codex`, `copilot`, or
  `cursor-agent`. Sage never auto-selects a cloud-capable CLI merely because it is installed.
- Command providers use fixed argv templates and `exec` directly; no shell parses model names,
  knowledge paths, or prompts.
- Stable `{model}` and `{kb}` placeholders resolve once. An empty model removes an adjacent
  `--model` or `-m` flag. `{prompt_text}` and `{prompt_file}` remain late-bound per request.
- File-mode prompts are created with private permissions under the Sage runtime root and removed
  after the request. Argument and stdin modes receive the same complete system/user prompt.
- A missing or unsupported explicitly selected backend is reported as unavailable. The daemon still
  starts, and durable model work remains retryable rather than being marked skipped.
- Command-model subprocesses inherit `DEVTRACK_SAGE_CAPTURE_OFF=1`; the silent hook honors that
  process-local switch without changing the persistent Sage pause marker, preventing recursion.
- Windows command providers request a hidden, no-console process. All providers remain bounded by
  the existing daemon-owned cancellation and `DEVTRACK_SAGE_MODEL_TIMEOUT_SECS` policy.

## Reference parity evidence

| Reference scenario | Go evidence | Result |
|---|---|---|
| `test_placeholders_are_rendered_and_empty_model_dropped` | `internal/sage/distill/backend_test.go#TestReferencePlaceholdersAreRenderedAndEmptyModelDropped` | implemented |
| `test_no_backend_when_binary_missing` | `internal/sage/distill/backend_test.go#TestReferenceNoBackendWhenBinaryMissing` | implemented |
| `test_shipped_backends_declare_a_prompt_mode` | `internal/sage/distill/backend_test.go#TestReferenceShippedBackendsDeclareAPromptMode` | implemented |
| `test_text_backend_defers_prompt_file_substitution` | `internal/sage/distill/backend_test.go#TestReferenceTextBackendDefersPromptFileSubstitution` | implemented |
| `test_env_switch_is_process_local` | `sage_cli_test.go#TestReferenceEnvSwitchIsProcessLocal` | implemented |

## Qualification

- Focused Go tests: root CLI, configuration, distillation, routing, and daemon integration passed.
- Full Go suite: `go test ./... -count=1 -timeout 180s` passed under Go 1.24 Docker.
- Static analysis: `go vet ./...` passed under Go 1.24 Docker.
- Windows/amd64 distillation test package cross-compiled successfully, including no-console flags.
- Final parity, documentation, shared-memory, and whitespace gates are recorded by the branch's
  docu-agent reconciliation.

No Python runtime dependency was added. No command provider is selected implicitly, no model call
occurs in the capture hook, and Sage never pushes knowledge commits.
