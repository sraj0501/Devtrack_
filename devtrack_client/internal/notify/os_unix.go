//go:build !windows

package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OS delivers a native OS notification: osascript on macOS, notify-send on Linux.
type OS struct{}

func (OS) Send(title, body, url string) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script).Run()
	case "linux":
		return exec.Command("notify-send", title, body).Run()
	default:
		return nil
	}
}
