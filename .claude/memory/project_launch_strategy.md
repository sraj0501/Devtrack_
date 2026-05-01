---
name: DevTrack Launch Strategy
description: Research-driven release and adoption plan — wedge feature, positioning rules, first-run UX targets, channel sequence, platform rollout order, risk register, and decision framework
type: project
---

Full strategy documented in `docs/LAUNCH_STRATEGY.md`. Key decisions:

**The wedge**: "DevTrack writes your standup update when you commit." — single feature at launch, everything else is docs-only until trust is earned.

**Why:** Research shows tools that launch with 15+ features fail to land a clear message; Swiss Army Knife trap. Cursor, Ruff, uv all won by being narrow first.

**How to apply:** Any new feature or messaging change should be evaluated against the decision framework at the bottom of LAUNCH_STRATEGY.md before shipping.

---

## Non-negotiable positioning rules

- Never use "AI-powered" in tagline or hero copy (triggers skepticism — AI trust at 29% in 2026)
- Never call it a "platform" or "all-in-one" before the user base exists
- Always lead with "runs on your machine, no account required"
- The local-first architecture is the structural moat — reinforce it, never compromise it

## First-run UX target

Time-to-value: **under 60 seconds**. No account, no T&C gate, no 12-var `.env` required before the first standup is generated.

## Platform integration rollout order

1. GitHub only (launch)
2. GitLab (after 100 stars)
3. Azure DevOps (after community traction)
4. Jira (only if ≥50 explicit user requests)

## Launch channel sequence

1. Pre-launch: Discord + Twitter/X building-in-public + Dev.to article
2. Launch day: Hacker News Show HN (GitHub link, Tuesday/Wednesday 9–11am ET)
3. Post-launch: console.dev, r/programming, awesome-cli-apps

## Decision framework (apply in order)

1. Does this serve the wedge?
2. Does this protect the local-first moat?
3. Does it reduce or increase time-to-value?
4. Is it premature expansion?
5. Who asked for it? (50+ requests = build it)
