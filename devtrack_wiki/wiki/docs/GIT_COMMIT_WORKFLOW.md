# DevTrack Enhanced Git Commit Workflow

## Overview

The new `devtrack git commit` workflow provides an AI-powered iterative commit message refinement experience with smart attempt tracking and flexible options.

---

## Two Modes of Operation

### 1. Default Mode (Interactive AI Enhancement)

**Command:** `devtrack git commit -m "message"`

This is the **primary workflow** that allows iterative refinement with up to 5 attempts.

```bash
$ devtrack git commit -m "added new authentication module"
```

**Flow:**
```
Staged changes preview
    ↓
AI analysis (Attempt 1/5)
    ↓
Show generated/enhanced message
    ↓
User chooses: Accept / Enhance / Regenerate / Cancel
    ↓
If not accepted → Return to step 2 (Attempt 2/5)
    ↓
If accepted → Create commit with final message
    ↓
Ask: Log this work? (optional logging)
```

### 2. Explicit Dry-Run Mode (AI Preview Only)

**Command:** `devtrack git commit -m "message" --dry-run`

This is a **preview-only mode** that shows AI enhancement capability without creating a commit or requiring interaction.

```bash
$ devtrack git commit -m "message" --dry-run
```

**Output:**
```
Staged changes preview
    ↓
AI analysis and enhancement
    ↓
Show AI-enhanced message suggestion
    ↓
Exit without interaction or commit
```

**Key difference from default:**
- Shows AI enhancement in action (demonstrates capability)
- No interactive options (Accept/Enhance/Regenerate)
- No commit created
- No user input required
- Perfect for reviewing what AI would suggest before committing

---

## Interactive Options per Attempt

At each attempt, you have 4 choices:

| Option | Key | Effect | Next Step |
|--------|-----|--------|-----------|
| **Accept** | `A` | Approve message and create commit | Commit created, ask to log |
| **Enhance** | `E` | Refine current message further | Regenerate with current message as input (next attempt) |
| **Regenerate** | `R` | Start fresh from original message | Regenerate with original message as input (next attempt) |
| **Cancel** | `C` | Abort without committing | Exit immediately |

---

## Attempt Tracking

The workflow displays clear attempt counters:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Attempt 1/5
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Key rules:**
- You get up to 5 attempts to accept a message
- Invalid inputs (wrong key) don't count toward your limit
- After attempt 5, only `[C]ancel` is offered
- If you reach attempt 5 without accepting, the process fails with an error

---

## Workflow Examples

### Example 1: Accept on First Attempt

```bash
$ devtrack git commit -m "fix bug in login"

Staged changes: [2 files changed]

✨ AI-Enhanced Commit Flow (up to 5 attempts)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Attempt 1/5
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔍 Analyzing with AI...
✓ AI-generated message:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Fix authentication bug in login flow

  Resolved issue where users were unable to log in due to
  improper token validation. Updated auth module and added
  additional test cases.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

What would you like to do?
  [A]ccept and commit
  [E]nhance/improve message
  [R]egenerate from scratch
  [C]ancel

Choice (A/E/R/C): a

✓ Committed successfully with AI-enhanced message!

🔔 DevTrack: Log this work? (y/n): y
✓ Logged work: Fix authentication bug in login flow
```

### Example 2: Regenerate Once, Then Accept

```bash
Choice (A/E/R/C): r

🔄 Regenerating from scratch...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Attempt 2/5
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ AI-generated message:
  [Different message from fresh AI analysis]

Choice (A/E/R/C): a  ← Accept the new version
```

### Example 3: Enhance (Iterative Refinement)

```bash
Choice (A/E/R/C): e

✨ Improving current message...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Attempt 2/5
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ AI-generated message:
  [Refined version of the previous message with improvements]

Choice (A/E/R/C): a  ← Accept the refined version
```

### Example 4: Max Attempts Reached

```bash
Attempt 5/5

What would you like to do?
  [A]ccept and commit
  [C]ancel (last attempt)

Choice (A/E/R/C): r

✗ Maximum attempts reached. Cannot regenerate.

Choice (A/E/R/C): c

✗ Commit cancelled.

# OR if you don't accept at attempt 5:

Choice (A/E/R/C): e

✗ Maximum attempts reached. Cannot enhance further.

# Then if you cancel:

✗ Maximum attempts (5) reached without acceptance.
✗ Commit cancelled.
```

### Example 5: Explicit Dry-Run (AI Preview)

```bash
$ devtrack git commit -m "test message" --dry-run

Staged changes: [test.txt added]

(preview mode: AI enhancement only, no commit, no interaction)

🔍 Analyzing with AI...
✓ AI-enhanced message:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Add test functionality

  Introduced test.txt to demonstrate new feature.
  This allows users to verify the system works correctly.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ Preview complete. No commit made.
Run without --dry-run to commit with interactive refinement.

# File is still staged. User can now:
# - Decide if they like the AI enhancement
# - Run without --dry-run to enter interactive loop
# - Or use git directly to skip AI entirely
```

---

## What Happens When

| Scenario | Result |
|----------|--------|
| Accept (A) at any attempt | ✅ Commit created immediately |
| Enhance (E) | ➡️ Next attempt with current message as input |
| Regenerate (R) | ➡️ Next attempt with original message as input |
| Cancel (C) | ❌ Exit without committing (files remain staged) |
| Reach attempt 5 without accepting | ❌ Process fails with error message |
| Invalid input (wrong key) | ⚠️ Shows error, doesn't count as attempt |
| Explicit `--dry-run` flag | 🔍 Shows AI enhancement, no commit, no interaction |
| AI enhancement fails | ⚠️ Shows fallback message, allows accept/enhance/regenerate anyway |

---

## Key Differences from Old Workflow

| Feature | Old | New |
|---------|-----|-----|
| Default behavior | Quick dry-run → require flag for AI | AI enhancement with attempts |
| Explicit dry-run | Via --dry-run flag | Still via --dry-run flag |
| Attempts allowed | 5 (but unclear) | 5 with clear "N/5" display |
| Enhance option | "Improve message" (changes input) | "Enhance" (uses current for next attempt) |
| Regenerate option | "Regenerate message" (changes input) | "Regenerate" (uses original for next attempt) |
| Max attempts handling | Silently failed | Fails with clear error message |
| User feedback | Basic | Detailed with attempt numbers and next-step hints |

---

## Tips & Best Practices

1. **Effective first messages**: Provide a meaningful `-m` message to get better AI suggestions
   - ❌ `devtrack git commit -m "fix"`
   - ✅ `devtrack git commit -m "fix bug in user authentication"`

2. **Use Enhance (E) for small tweaks**: When the message is 90% good but needs minor refinement

3. **Use Regenerate (R) for fresh perspectives**: When you want AI to try a completely different approach

4. **Use --dry-run for preview**: Before committing with AI, preview what will be staged
   ```bash
   devtrack git commit -m "message" --dry-run  # Preview
   devtrack git commit -m "message"            # Actual commit with AI
   ```

5. **All attempts use the same staged changes**: Switching between Enhance/Regenerate doesn't change what's being committed, just the message

---

## Error Messages

| Error | Meaning | Solution |
|-------|---------|----------|
| "No changes staged" | No files are staged | Use `git add` to stage files first |
| "Maximum attempts (5) reached" | Hit the 5-attempt limit | Accept a message or cancel and try again |
| "Invalid choice" | Wrong key pressed | Press A, E, R, or C |
| "Commit cancelled" | User pressed C or workflow failed | Files remain staged for next attempt |

---

## Architecture Notes

The wrapper script manages:
- **Attempt counting**: Tracks which attempt (1-5) you're on
- **Message state**: Keeps track of current message vs. original message
- **Input routing**: Maps E/R correctly to next attempt's input
- **Fallback handling**: If AI unavailable, shows message anyway
- **Clean exit**: Preserves staged changes on cancel
