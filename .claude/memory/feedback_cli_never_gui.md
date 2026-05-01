---
name: devtrack CLI is always CLI/TUI — never GUI
description: Strong design constraint: the Go devtrack binary must never become a GUI app
type: feedback
---

The devtrack Go CLI will ALWAYS be a CLI or TUI tool. Adding a GUI would break the complete premise of the software — it is built for developers who live in the terminal.

**Why:** The core value proposition is terminal-first workflow automation. A GUI would make it just another project management tool.

**How to apply:** Never suggest a GUI, web UI, Electron app, or desktop interface for the devtrack Go binary. TUI enhancements (Bubble Tea) are fine. The GUI lives on the Python server side only (admin console for server management — not for developers using devtrack day-to-day).
