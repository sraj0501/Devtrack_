//go:build !windows

package main

import (
	"os/exec"
	"testing"
)

func TestReferencePosixHelpersDoNotReceiveWindowsFlags(t *testing.T) {
	cmd := exec.Command("true")
	setSetsid(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("POSIX detached process did not request a new session")
	}
}
