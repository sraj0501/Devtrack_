//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// elevatedReplace retries the binary copy via sudo, prompting for a password.
// Used when the destination (e.g. /usr/local/bin) is root-owned.
func elevatedReplace(dst, src string) error {
	fmt.Println("  Needs elevated permissions — retrying with sudo...")
	sudoCp := exec.Command("sudo", "cp", src, dst)
	sudoCp.Stdin = os.Stdin
	sudoCp.Stdout = os.Stdout
	sudoCp.Stderr = os.Stderr
	if err := sudoCp.Run(); err != nil {
		return fmt.Errorf("sudo cp failed — try manually: sudo cp %s %s", src, dst)
	}
	_ = exec.Command("sudo", "chmod", "755", dst).Run()
	return nil
}
