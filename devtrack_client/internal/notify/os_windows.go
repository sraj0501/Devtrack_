//go:build windows

package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// OS delivers a Windows toast notification via PowerShell.
type OS struct{}

func (OS) Send(title, body, url string) error {
	msg := title
	if body != "" {
		msg += " — " + body
	}
	// Use PowerShell BurntToast-compatible command if available,
	// otherwise fall back to a simple balloon via msg.exe.
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.BalloonTipTitle = %s
$n.BalloonTipText  = %s
$n.Visible = $true
$n.ShowBalloonTip(5000)
Start-Sleep -Milliseconds 500
$n.Dispose()
`, powershellQuote(title), powershellQuote(msg))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("notify/os: %w", err)
	}
	return nil
}

func powershellQuote(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}
