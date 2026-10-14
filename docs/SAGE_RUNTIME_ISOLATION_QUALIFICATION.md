# Sage runtime and knowledge isolation qualification

Date: 2026-10-13  
Task: TASK-157 / SAGE-003  
Branch: `features/TASK-157-sage-backend-isolation`  
Reference: `C:\Work\git_apps\ai_sessions_skills` at `ea2e067`

## Delivered behavior

- `DEVTRACK_SAGE_RUNTIME_DIR` selects private Sage queue, state, and diagnostic storage. Relative
  values resolve from the stable Sage data root rather than the process working directory.
- `DEVTRACK_SAGE_KNOWLEDGE_DIR` selects portable Markdown output independently. Relative values
  resolve from the same stable Sage root; an explicit knowledge override never relocates private
  runtime state.
- Daemon startup and Sage CLI commands use the same runtime-root resolver.
- The Go distillation prompt contains only sanitized captured facts and existing section names. It
  does not expose queue, database, runtime, or knowledge paths. This is a stronger Go-native
  adaptation of the reference agent prompt's separate runtime batch path.

## Reference parity evidence

| Reference scenario | Go evidence | Result |
|---|---|---|
| `test_agent_prompt_uses_separate_runtime_folder` | `internal/sage/distill/distill_test.go#TestReferenceAgentPromptUsesSeparateRuntimeFolder` | adapted: prompt is path-free |
| `test_runtime_setting_is_relative_to_config` | `internal/config/sage_paths_test.go#TestReferenceRuntimeSettingIsRelativeToConfig` | implemented |
| `test_explicit_kb_override_keeps_runtime_isolated` | `internal/config/sage_paths_test.go#TestReferenceExplicitKbOverrideKeepsRuntimeIsolated` | implemented |

An additional regression, `TestRelativeKnowledgeOverrideIsNotWorkingDirectoryRelative`, proves
that relative knowledge placement is also independent of the caller's working directory.
`TestRuntimeOverrideDoesNotMoveDefaultKnowledgeRoot` proves that selecting a private runtime
subdirectory does not move the default Markdown directory underneath it.

## Qualification

- Focused Go tests: `go test ./internal/config ./internal/sage/distill ./internal/infra -count=1`
- Full Go suite: `go test ./... -count=1 -timeout 180s`
- Static analysis: `go vet ./...`
- Parity ledger against `ea2e067`: 136 rows; 84 implemented, 2 adapted, 2 partial, 48 pending
- Parity-checker unit tests: 8 passed
- Shared-memory and whitespace validation: passed

Go 1.24 qualification ran in the documented Docker environment because host Go is not installed.
No Python runtime dependency was added to DevTrack Sage.
