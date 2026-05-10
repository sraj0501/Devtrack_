# build-runner — Design & Implementation Plan

> **Status: Deferred** — fully designed, not yet built. Tasks BR-001–BR-009 are on the project board.
> When ready: create `gitlab.com/devtrack3_cloud/build-runner` on GitLab, then dispatch the engineer.

---

## Problem

GitLab-hosted CI runners are consuming compute credits at an unsustainable rate, primarily from cross-platform Go binary builds. The fix is to move all builds to a self-hosted Docker container on the developer's laptop — zero cloud compute cost, same workflow.

---

## Goal

A standalone, language-agnostic build and release platform consisting of:

1. **Docker image hierarchy** published to `registry.gitlab.com/devtrack3_cloud/build-runner`
2. **Shared GitLab CI pipeline template** that any project can `include:`
3. **Self-hosted GitLab runner** running in Docker locally — all CI jobs run on your machine

---

## Repository

**GitLab repo**: `gitlab.com/devtrack3_cloud/build-runner` (public, standalone — not a subtree of the DevTrack monorepo)

**Registry**: `registry.gitlab.com/devtrack3_cloud/build-runner:<tag>`

---

## Image Hierarchy

```
build-runner:base
  curl, jq, zip, bash, git
  glab CLI       — GitLab releases + API
  gh CLI         — GitHub fallback
  netlify-cli    — static site deploys
  node + npm     — required by netlify-cli
        │
        ├── build-runner:go        base + Go toolchain (cross-compile: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
        ├── build-runner:python    base + Python + uv
        ├── build-runner:node      base + Node LTS
        └── build-runner:rust      base + Rust + cargo  [low priority, implement last]
```

All tool versions are **pinned** — no `latest` tags anywhere. Each image versioned independently (e.g. `go-1.22`, `python-3.12`).

---

## Language-Agnostic Pipeline Contract

The image provides tools. The **consuming project** provides scripts:

```
any-project/
  scripts/
    build.sh       # compile/package — writes artifacts to $OUTPUT_DIR
    release.sh     # upload to GitLab Releases + any deploy step
    test.sh        # (optional) run tests before release
```

### Standard env vars injected by the runner

| Var | Value |
|---|---|
| `PROJECT` | repo name |
| `VERSION` | `$CI_COMMIT_TAG` or `dev` |
| `OUTPUT_DIR` | `/workspace/bin` (volume-mounted to host) |
| `GLAB_TOKEN` | GitLab API token (secret) |
| `NETLIFY_AUTH_TOKEN` | for projects that deploy a static site |

Any language, any project — same three script hooks, same vars.

---

## Shared Pipeline Template

Projects include one line instead of copy-pasting CI config:

```yaml
# consuming project's .gitlab-ci.yml
include:
  - project: devtrack3_cloud/build-runner
    file: templates/pipeline.yml

variables:
  BUILD_IMAGE: registry.gitlab.com/devtrack3_cloud/build-runner:go-1.22
```

### `templates/pipeline.yml` — job definitions

```yaml
stages: [test, build, release]

.build-base:
  image: $BUILD_IMAGE
  tags: [self-hosted]        # always runs on your local Docker runner

build-dev:
  extends: .build-base
  stage: build
  rules:
    - if: $CI_COMMIT_BRANCH == "dev"
  script:
    - scripts/build.sh
  # artifacts land on host via Docker volume mount — nothing goes public

release:
  extends: .build-base
  stage: release
  rules:
    - if: $CI_COMMIT_TAG =~ /^v/
  script:
    - scripts/build.sh
    - scripts/release.sh     # uploads to GitLab Releases + deploys site if applicable
```

---

## Two-Tier Workflow (what consuming projects get)

### Tier 1 — `dev` push (local test build)
Push to `dev` → self-hosted runner cross-compiles all platforms → binaries appear in local volume mount on host → developer tests locally → **nothing goes public**

### Tier 2 — `v*` tag on `main` (public release)
Tag pushed → runner builds final binaries → `glab release create` uploads to GitLab Releases (public, free) → `netlify deploy --prod` if project has a website → install scripts and download page serve the new version

---

## Repo Structure to Build

```
build-runner/
  Dockerfile.base              # base image — all release tooling
  Dockerfile.go                # Go cross-compile layer
  Dockerfile.python            # Python + uv layer
  Dockerfile.node              # Node LTS layer
  Dockerfile.rust              # Rust + cargo [stub only — implement later]
  templates/
    pipeline.yml               # shared CI template for consuming projects
  scripts/
    build-all.sh               # example cross-compile loop (parameterised)
    publish.sh                 # builds and pushes all images to registry
    setup-runner.sh            # registers self-hosted GitLab runner locally
  .gitlab-ci.yml               # the repo's own pipeline: builds + publishes images on tag push
  README.md                    # usage docs: how to adopt in a new project
```

---

## Self-Hosted Runner Setup

The runner is a Docker container on the developer's laptop:

- `scripts/setup-runner.sh` handles `gitlab-runner register` with the correct config
- Uses `docker` executor — each job runs in a fresh container
- Mounts a local `/releases/` volume so build artifacts are accessible on the host after the job completes
- Start once, stop when not building

### Hybrid approach (recommended)

- **Automated**: Self-hosted GitLab runner picks up jobs on `dev` push and `v*` tag automatically
- **Manual fallback**: `scripts/release.sh v2.x.x` — docker run + build + upload — for hotfixes or when runner is stopped

---

## Deployment Architecture (DevTrack-specific context)

This replaces the current DevTrack CI setup:

| Component | Before | After |
|---|---|---|
| Binary builds | GitLab-hosted runners (costs credits) | Self-hosted Docker runner (zero cost) |
| Binary hosting | GitLab Package Registry | GitLab Releases (public repo, free) |
| Website deploy | GitLab CI → Netlify API | Netlify CLI from runner |
| GitHub | Passive backup | Source of truth (private — contains sensitive configs) |
| GitLab | CI + hosting | Mirror only + public binary hosting |

**Key constraint**: GitHub repo is private (contains Claude memory files, configs). GitHub Release assets on a private repo are NOT publicly accessible. GitLab is public — GitLab Releases are the correct public distribution point.

---

## Task Breakdown (project board: BR-001 – BR-009)

| Task | Title | Depends on |
|---|---|---|
| BR-001 | Repo bootstrap: git init, branch structure, stub files | none |
| BR-002 | Dockerfile.base: pinned tools | BR-001 |
| BR-003 | Dockerfile.go: Go toolchain + cross-compile | BR-002 |
| BR-004 | Dockerfile.python: Python + uv | BR-002 |
| BR-005 | Dockerfile.node: Node LTS | BR-002 |
| BR-006 | templates/pipeline.yml: shared CI template | BR-002, BR-003 |
| BR-007 | .gitlab-ci.yml + scripts/publish.sh: repo's own publish pipeline | BR-002–BR-005 |
| BR-008 | scripts/setup-runner.sh: self-hosted runner registration | BR-001, BR-002 |
| BR-009 | README.md: full usage documentation | BR-002–BR-008 |

**Parallelism**: After BR-001 + BR-002, streams BR-003/BR-005/BR-006/BR-008 can all run in parallel.

---

## Future Scaling Path

When traffic or team size grows, revisit in this order:

1. **Binary traffic increases** → Move from GitLab Releases to Cloudflare R2 (free egress, S3-compatible)
2. **Need automated builds without laptop** → Migrate runner to a small VPS/EC2
3. **Moving off GitHub** → Mirror to self-hosted Forgejo; build-runner stays identical (GitLab runner protocol is compatible)
4. **Team grows** → Formalise branching strategy, add PR-based deploy previews

---

## To Resume This Work

1. Create `gitlab.com/devtrack3_cloud/build-runner` on GitLab (public repo)
2. Tell Claude: _"resume the build-runner plan"_ — tasks BR-001–BR-009 are on the project board, spec is in this file
3. Engineer will clone the new repo and build out BR-001 first
