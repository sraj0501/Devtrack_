//go:build windows

package daemon

import (
	"os/exec"
	"strconv"

	"golang.org/x/sys/windows"
)

// KillProcessTree terminates pid and all of its descendants.
// The webhook server is spawned as "uv run ... python -m backend.webhook_server":
// uv.exe and python.exe are two distinct processes on Windows (no exec-replace
// like on POSIX), so Process.Kill() on the tracked *os/exec.Cmd only kills uv.exe
// and leaves python.exe running as an orphan — still bound to its port, still
// holding its log file handle. Verified live: after 'devtrack stop', python.exe
// kept LISTENING on the webhook port. taskkill /T kills the whole tree.
func KillProcessTree(pid int) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// CheckProcessAlive uses the Windows OpenProcess API to check if a PID is alive.
// signal(0) doesn't work on Windows — it always returns EWINDOWS regardless.
func CheckProcessAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}
