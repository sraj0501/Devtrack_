---
name: PR target branch rule
description: PRs must always target dev, never main. The flow is feature branch → dev → main.
type: feedback
originSessionId: 224dbc1c-de8b-4635-b8b1-b826b4c85afa
---
All PRs raised by the engineer or PM must target `dev`, never `main`. Use `--base dev` on every `gh pr create` call.

**Why:** `main` is the release branch. `dev` is the integration branch. PRs going directly to `main` skip the integration gate and bypass the standard review flow. The developer explicitly reinforced this rule on 2026-04-23 after the PM agent opened PR #79 directly to `main`.

**How to apply:** Every `gh pr create` must include `--base dev`. Before creating any PR, verify the base branch is `dev`. Promoting `dev` → `main` is a separate, explicit developer action — never initiated by the engineer or PM autonomously.
