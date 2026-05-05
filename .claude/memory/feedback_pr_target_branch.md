---
name: MR target branch rule
description: MRs must always target dev, never main. The flow is feature branch → dev → main.
type: feedback
---
All MRs raised by the engineer or PM must target `dev`, never `main`. Use `--target-branch dev` on every GitLab MR (or `--base dev` for `gh`).

**Why:** `main` is the release branch. `dev` is the integration branch. MRs going directly to `main` skip the integration gate and bypass the standard review flow. The developer explicitly reinforced this rule on 2026-04-23 after the PM agent opened PR #79 directly to `main`.

**How to apply:** Always verify the target/base branch is `dev` before creating an MR. Promoting `dev` → `main` is a separate, explicit developer action — never initiated by the engineer or PM autonomously.
