package main

// tui_shim.go — forwarding alias from package main to internal/tui.
// The Bubble Tea dashboard lives in internal/tui; the CLI launches it via RunTUI.

import (
	tui "github.com/sraj0501/Devtrack_/devtrack_client/internal/tui"
)

// RunTUI opens the Bubble Tea TUI dashboard.
func RunTUI() error { return tui.RunTUI() }
