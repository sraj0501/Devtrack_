# DevTrack TUI Flows

DevTrack has two interactive UIs that serve different purposes.

## 1. Go TUI (Bubble Tea) – Standalone Menu

**When**: Run `devtrack` with no arguments (no subcommand).

**Purpose**: Interactive menu for ad-hoc tasks:
- Parse daily update from text
- Update MS Lists
- Generate Email
- Create Subtasks
- Exit

**Flow**: User selects an option, enters text in a textarea, and the Go process invokes Python scripts (e.g. `backend/ai/create_tasks.py`) directly. No daemon required.

**File**: `devtrack_client/tui.go`

---

## 2. Python TUI (user_prompt.py) – Daemon-Triggered Prompts

**When**: Daemon is running and a **timer trigger** or **commit trigger** fires. The Python bridge receives the trigger via IPC and prompts the user.

**Purpose**: Capture work updates in context:
- Timer: "What have you been working on?" (scheduled or `devtrack force-trigger`)
- Commit: Parse commit message, optionally prompt for confirmation

**Flow**:
1. Go daemon detects event (git commit or scheduler)
2. Daemon sends IPC message to Python bridge
3. Python bridge calls `DevTrackTUI.prompt_work_update()` or similar
4. User input is enriched through the configured LLM and returned as a task update
5. Go persists to SQLite

**File**: `devtrack_server/backend/user_prompt.py`, used by `webhook_server.py`

**Non-interactive**: When stdin is not a TTY (CI/automation), set `DEVTRACK_INPUT` env var to provide input without prompting.

---

## Summary

| Flow        | Trigger        | TUI Location | Daemon Required |
| ----------- | -------------- | ------------ | ----------------- |
| Standalone  | `devtrack`     | Go (tui.go)  | No               |
| Timer/Commit| IPC from daemon| Python       | Yes              |

Both flows support "interactive work updates" as described in the wiki. The Go TUI is for manual, one-off updates; the Python TUI is for automated, context-aware prompts when the daemon is running.
