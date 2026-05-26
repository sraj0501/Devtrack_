# DevTrack Project Board

_Last updated: 2026-05-24 — EPIC-SPLIT complete; next: TASK-050 (GitLab cut-over)_
_Next DevTrack task ID: TASK-051_
_Next build-runner task ID: BR-010_

---

## NEXT — pick up here

### TASK-050 — GitLab cut-over (DEVELOPER ACTION, no code changes)
**Priority**: HIGH | **Branch**: `features/SPLIT-001-monorepo-restructure`

1. Run full e2e test: start daemon, make a commit, verify trigger reaches server
2. Open PR: `features/SPLIT-001-monorepo-restructure` → `dev` on GitHub
3. After merge: `git subtree push --prefix=devtrack_client/ gitlab-client dev` and same for `devtrack_server/`
4. Verify GitLab CI green on both repos
5. Swap `ci/devtrack_server_new.gitlab-ci.yml` → `ci/devtrack_server.gitlab-ci.yml`

**Not dispatched to engineer** — pure git push + pipeline verification.

---

### TASK-049 — Rewrite devtrack_wiki docs (DEFERRED)
**Priority**: LOW | **Branch**: new from `dev` after TASK-050 merges
Wiki docs are out of date but not blocking. Revisit after cut-over is stable.

---

### After TASK-050: delete `devtrack_contract` repo on GitLab (dead repo, no code)

---

## BUILD-RUNNER (standalone repo — gitlab.com/devtrack3_cloud/build-runner)

All BR tasks deferred. Spec detail at `docs/build-runner-plan.md`. Create GitLab repo first.

| Task | Description | Priority |
|---|---|---|
| BR-001 | Repo bootstrap: git init, branch structure, .gitignore, stubs | HIGH |
| BR-002 | Dockerfile.base: pinned debian, curl/jq/git/glab/gh/node/netlify-cli | HIGH |
| BR-003 | Dockerfile.go: Go toolchain + 5-platform cross-compile | HIGH |
| BR-004 | Dockerfile.python: Python + uv layer | HIGH |
| BR-005 | Dockerfile.node: Node LTS + yarn + pnpm | MEDIUM |
| BR-006 | templates/pipeline.yml: shared CI template (build-dev / test / release jobs) | HIGH |
| BR-007 | .gitlab-ci.yml + scripts/publish.sh: publish images on v* tag | HIGH |
| BR-008 | scripts/setup-runner.sh: self-hosted GitLab runner registration | HIGH |
| BR-009 | README.md: full usage documentation | MEDIUM |

---

## BLOCKED — deploy-arch (waiting on TASK-050 merge to dev)

| Task | Description |
|---|---|
| TASK-034 | Disable GitLab CI pipelines on all three repos (stop compute drain) |
| TASK-035 | Install and configure Netlify CLI locally for manual deploys |
| TASK-036 | Create local binary build + GitHub Release upload script (`scripts/release.sh`) |
| TASK-037 | Update download page and install scripts to use GitHub Release URLs |
| TASK-038 | Verify GitHub private repo Release assets are publicly accessible |
| TASK-039 | Document the manual release process (`docs/RELEASE_PROCESS.md`) |

Key open question before TASK-037/038: is `sraj0501/Devtrack_` public or private?
If private, Release assets need separate public repo or R2 bucket.

---

## PLANNED — Other

- **TASK-026** (LOW): Remove `GetPythonBridgePath` dead code from `config_env.go`

---

## Platform Strategy (2026-04-05)

Linux-first for server-side code. Go binary is cross-platform. macOS = dev machine. Windows = stretch.

---

## DONE (summary)

### EPIC-SPLIT (2026-05-24, branch `features/SPLIT-001-monorepo-restructure`)

| Task | What |
|---|---|
| TASK-041 | Audit + split-manifest.md: every file catalogued by owner |
| TASK-042 | `devtrack_client/` skeleton (Go files + git_sage copy); go build/vet/test pass |
| TASK-043 | `devtrack_server/` skeleton (Python backend copy); pytest 549 pass |
| TASK-044 | `docs/HTTP_API.md` + Go + Python API contract tests |
| TASK-045 | `.github/workflows/client.yml` (GitHub Actions, 3-job, matrix cross-compile) |
| TASK-046 | `ci/devtrack_server_new.gitlab-ci.yml` (GitLab CI, 4-job, uv sync + docker) |
| TASK-047 | CLAUDE.md + README + docs/ARCHITECTURE.md updated for three-codebase split |
| TASK-048 | Retired `devtrack-bin/`, root `backend/`, `bin/`, `demo/`, `python_bridge.py` (281 files, 69k lines) |

### Earlier sessions (2026-05-09 and before)

| Task | What |
|---|---|
| TASK-040 | DevTrack logo in website nav/footer + Windows binary icon via goversioninfo |
| TASK-029–033 | pyproject.toml two-tier deps; is_ai_available(); devtrack-server features/enable cmds; GitLab CI core+full jobs |
| TASK-026–028 | Remove dead GetPythonBridgePath; guard work-report in Lightweight mode; internal HTTP control API + cross-platform AlertNotifier |
| TASK-025 | Windows native build: build-tag syscall split (daemon_unix/windows, cli_unix/windows) |
| TASK-021–024 | setup.go mode wizard; Lightweight mode skips Python; capability guards; non-fatal config accessors |
| TASK-018–020 | CS-3 hardcoded value audit (high/med/low); inbound webhook integration tests |
| TASK-011–015 | CS-3 Admin GUI MVP: route tests, role/disable, license page, trigger stats, polish+embed |
| TASK-010 | Full documentation + memory audit |
| TASK-007–009 | CS-2 headless tests (37); stats panel; os.getenv remaining fixes |
| TASK-001–006 | Config audit: 50+ config accessors, all os.getenv eliminated across 22 files |
| TASK-000 | v1.0.0 release + local agents setup |
