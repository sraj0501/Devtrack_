//go:build windows

package main

import (
	"fmt"
	"os"
)

// elevatedReplace handles the case where the direct rename-over-dst failed.
// On Windows, a running .exe cannot be overwritten even as Administrator because
// the OS maps the file into memory by handle. The workaround is to rename the
// old binary aside first (Windows allows renaming in-use files), then copy the
// new binary into place under the original name.
func elevatedReplace(dst, src string) error {
	old := dst + ".old"
	_ = os.Remove(old) // clean up any leftover from a previous upgrade attempt
	if err := os.Rename(dst, old); err != nil {
		return fmt.Errorf(
			"upgrade failed — could not rename running binary: %w\n\n"+
				"Stop devtrack first (devtrack stop), then re-run: devtrack upgrade",
			err,
		)
	}
	if err := copyFile(src, dst); err != nil {
		_ = os.Rename(old, dst) // restore original so devtrack still works
		return fmt.Errorf("upgrade failed — copy new binary: %w", err)
	}
	_ = os.Chmod(dst, 0755)
	// .old may still be held open by the running process; silently ignore removal failure.
	_ = os.Remove(old)
	return nil
}
