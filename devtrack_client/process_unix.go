//go:build !windows

package main

import (
	"os"
	"syscall"
)

func checkProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
