//go:build windows

package main

import (
	"fmt"
	"os"
)

// removeSelfBinary on Windows cannot delete a running .exe (the OS holds it open).
// We rename it to .old so the directory entry is freed; the .old file remains
// until the next reboot or manual deletion.
func removeSelfBinary(path string) error {
	old := path + ".old"
	_ = os.Remove(old)
	if err := os.Rename(path, old); err != nil {
		return fmt.Errorf("cannot remove running binary — delete manually after stopping devtrack:\n  del \"%s\"", path)
	}
	fmt.Printf("  (renamed to %s — safe to delete after reboot)\n", old)
	return nil
}
