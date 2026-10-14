//go:build !windows

package distill

import "os/exec"

func configureBackgroundCommand(_ *exec.Cmd) {}
