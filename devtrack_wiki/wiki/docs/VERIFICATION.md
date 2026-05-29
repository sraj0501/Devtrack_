# DevTrack Manual Verification Guide

This document describes the exact steps to manually verify that DevTrack's basic features work correctly.

## Prerequisites

1. **Environment**: Copy `.env_sample` to `.env` and fill in your paths (especially `PROJECT_ROOT`, `DEVTRACK_WORKSPACE`).
2. **Dependencies**: Run `./scripts/verify_setup.sh` to verify setup.
3. **Unit tests**: Run `uv run pytest backend/tests/ -v` - all tests should pass.

---

## Step 1: Verify Daemon Starts

```bash
# From project root
devtrack start
```

**Expected**: Daemon starts, no error messages. You should see "Daemon started successfully" or similar in the output.

---

## Step 2: Verify Daemon and Python Bridge Running

```bash
devtrack status
```

**Expected**: Status shows "Running" with PID. The Python bridge process should also be running (check with `ps aux | grep python_bridge`).

---

## Step 3: Verify Git Commit Triggers Processing

1. Ensure daemon is running (`devtrack start`).
2. Make a commit in your workspace (the path in `DEVTRACK_WORKSPACE`):

   ```bash
   cd $DEVTRACK_WORKSPACE  # or your project path
   git add -A  # or add specific files
   git commit -m "Test commit for DevTrack verification"
   ```

3. Wait 5-10 seconds (git monitor polls every 2 seconds).
4. Check daemon logs:

   ```bash
   devtrack logs | tail -50
   ```

**Expected**: Log should contain "New commit detected" and "Sent trigger to Python via IPC". If Python bridge is connected, you should also see "Commit processing complete".

---

## Step 4: Verify Preview Report

```bash
devtrack preview-report
```

**Expected**: Command runs without error. Output may be empty if no activities are in the database, or show a formatted report if data exists.

---

## Step 5: Stop Daemon

```bash
devtrack stop
```

**Expected**: Daemon stops cleanly.

---

## Automated Verification Scripts

| Script | Purpose |
|-------|---------|
| `scripts/verify_setup.sh` | Verify .env, uv sync, spaCy, Go build |
| `scripts/test_commit_enhancer.sh` | Test commit message enhancer with staged changes (run in normal terminal; may fail in restricted sandbox due to .git writes) |
| `scripts/test_commit_flow.sh` | Test full daemon + commit detection flow |
| `scripts/test_force_trigger.sh` | Test force-trigger (SIGUSR2 → daemon → Python) |
| `scripts/test_preview_report.sh` | Test report preview (preview-report command) |
| `scripts/test_integrations.sh` | Optional: Azure/GitHub (skip if tokens not set), task_matcher, learning_integration |
| `scripts/test_ipc_manual.py` | Test IPC connectivity (daemon must be running) |

Run unit tests before any verification:

```bash
uv run pytest backend/tests/ -v
```

See `docs/TUI_FLOWS.md` for documentation of the Go TUI (standalone menu) vs Python TUI (daemon-triggered prompts).
