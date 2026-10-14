# Sage platform layout and installer qualification

Date: 2026-10-14  
Task: TASK-157 / SAGE-002  
Branch: `features/TASK-157-sage-platform-installers`  
Reference: `C:\Work\git_apps\ai_sessions_skills` at `ea2e067`

## Delivered behavior

- Pure target-platform path resolution covers Windows `APPDATA` with home fallback, Linux XDG with
  `.config` fallback, and the reference macOS `.config` convention.
- Devin follows the platform configuration root. Claude, Codex, Copilot, and Cursor keep their
  home-relative dot directories on every platform. Windows OpenCode respects an existing
  `~/.config/opencode` directory before creating an AppData location.
- Every reference harness has a deterministic Markdown skill destination and frontmatter style.
  These paths do not advertise an adapter: the selective harness registry remains the authority
  for currently installable integrations, and Codex remains the only shipped capture adapter.
- DevTrack's packaged Go executable replaces the reference Python `.cmd` and `.sh` hook shims.
  Windows uses `devtrack.exe`; Linux and macOS use `devtrack`. This preserves native execution
  without adding Python to the Sage runtime.
- Codex hook installation recursively removes only entries containing DevTrack's owner marker or
  a known `ai_sessions_skills` / `ai-session-skills` legacy path marker before adding one current
  entry. Foreign settings and hooks remain intact.
- Reinstall is byte-stable and reports `unchanged`. Install/removal also report `created`,
  `updated`, or `missing`, so an absent removal target is distinguishable without creating files.

## Reference parity evidence

| Reference group | Exact Go evidence | Result |
|---|---|---|
| User configuration roots | `internal/sage/hooks/layout_test.go#TestReferenceWindowsUsesAppdata` through `TestReferenceMacosDefaultIsDotConfig` | 5 implemented |
| Harness layouts and skill destinations | `internal/sage/hooks/layout_test.go#TestReferenceLinuxDevinUnderXdg` through `TestReferenceEveryHarnessHasASkillDestination` | 6 implemented |
| Platform launcher scripts | `internal/sage/hooks/layout_test.go#TestReferenceLauncherExtensionMatchesPlatform`; `TestReferenceBothPlatformShimsAreShipped` | 2 adapted to native Go launchers |
| Additive install/removal | `internal/sage/hooks/install_test.go#TestReferenceWindowsPathsDoNotDuplicateHooksOnReinstall` through `TestReferenceMissingFileReportsMissingOnRemove` | 5 implemented |

## Qualification

- Focused hooks and harness tests passed under Go 1.24 Docker.
- Full suite: `go test ./... -count=1 -timeout 180s` passed under Go 1.24 Docker.
- Static analysis: `go vet ./...` passed under Go 1.24 Docker.
- The hooks package cross-compiled for Windows/amd64.
- The parity checker against current reference HEAD passes: 136 scenarios; 105 implemented,
  4 adapted, 2 partial, and 25 pending.
- Final shared-memory, wiki, parity-checker, and whitespace evidence is recorded after the two
  docu-agent reconciliation passes.

No additional harness is advertised or installed by this slice, and no Python runtime dependency
was introduced.
