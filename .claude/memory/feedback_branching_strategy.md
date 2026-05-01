---
name: Branching Strategy
description: Git branching rules for DevTrack — all changes go to dev first, dev PRs to main (prod). Never push directly to main.
type: feedback
---

All changes must go to `dev` first. `dev` then creates a PR to `main` (production). No direct pushes to `main` are allowed.

**Why:** The project is moving toward a SaaS model. `dev` is the integration branch; `main` is prod. This protects production from unreviewed changes.

**How to apply:**
- Any feature branch work → merge/push to `dev`, not `main`
- When suggesting git workflows or PRs, always target `dev`
- Remind the user if they ask to push directly to `main`
- The flow is: `feature branch` → `dev` → PR → `main`
