//go:build windows

package daemon

import "golang.org/x/sys/windows"

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
