---
name: TUI Navigation Design
description: ESC = go back one step, Ctrl+C = cancel entire flow. FlowController + RawInput architecture for all DevTrack TUI flows.
type: project
---

Full design discussion recorded April 1, 2026. Not yet implemented.

**The problem**: Current `user_prompt.py` uses Python's line-buffered `input()` — ESC is never detected as a standalone keypress, it just cancels the flow. There is no concept of a previous step anywhere in the architecture.

**The rule going forward**: Every multi-step TUI flow must use `FlowController`. Every prompt must return a `StepResult`. ESC = back one step. Ctrl+C = cancel the whole flow.

---

## Key Design Decisions

**Navigation contract** (universal, no exceptions):
- `ESC` → go back to the previous step
- `Ctrl+C` → cancel the entire flow immediately
- If ESC is pressed on the first step → stay on step 1, show: "Already at the start. Ctrl+C to cancel."

**Why:** Developers expect ESC to mean "undo last action" in terminal UIs (vim, fzf, git interactive). Killing the entire flow on ESC is a destructive surprise, especially mid-commit.

**How to apply:** Any new TUI flow must be built with `FlowController` from the start. Never chain bare `prompt_user()` calls for multi-step flows.

---

## New Classes (to be added to `backend/user_prompt.py`)

### `StepResult` (replaces raw `UserResponse` for flow steps)
```python
@dataclass
class StepResult:
    action: Literal["forward", "back", "cancel"]
    data: Any = None   # populated only when action="forward"
```

### `RawInput`
- Puts terminal in raw mode (`termios.setraw`)
- Reads chars one at a time
- `\x03` (Ctrl+C) → returns `StepResult(action="cancel")`
- `\x1b` alone (ESC, < 50ms timeout for next byte) → returns `StepResult(action="back")`
- `\x1b[A/B/C/D` → arrow key sequences (navigation within a list)
- `\r` / `\n` → `StepResult(action="forward", data=accumulated_text)`
- Handles backspace, delete, cursor movement
- Always restores terminal on exception (critical)
- Windows fallback: line-buffered input (no raw mode)

### `FlowController`
```python
FlowController(steps=[step1_fn, step2_fn, step3_fn]).run()
# Returns dict of results on completion, None on Ctrl+C cancel
```
- Runs steps in sequence
- On `back`: decrements step index, clears that step's result, re-runs
- On `cancel`: shows "Cancelled." and returns None
- On `forward`: stores result, advances to next step

---

## Backward Compatibility

`UserResponse` gets a new `went_back: bool = False` field. Existing callers that don't check it are unaffected. `_prompt_simple` and `_prompt_with_timeout` swap `input()` for `RawInput.readline()` — the only change to existing methods.

## Files to Change

- `backend/user_prompt.py` — add `StepResult`, `RawInput`, `FlowController`; add `went_back` to `UserResponse`; swap `input()` for `RawInput`
- `backend/git_sage/cli.py` — update approval dialog to use `FlowController`
- All future multi-step TUI flows — use `FlowController` from the start
