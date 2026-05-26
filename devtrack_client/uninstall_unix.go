//go:build !windows

package main

import "os"

// removeSelfBinary deletes the binary at path.
// On Unix, deleting a running executable is allowed — the OS keeps the inode
// alive until the process exits, so the running process is unaffected.
func removeSelfBinary(path string) error {
	return os.Remove(path)
}
