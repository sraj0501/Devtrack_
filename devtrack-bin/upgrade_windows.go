//go:build windows

package main

import "fmt"

// elevatedReplace is called when writing the new binary is denied.
// Windows has no sudo; instruct the user to re-run as Administrator.
func elevatedReplace(dst, src string) error {
	return fmt.Errorf(
		"permission denied — re-run as Administrator, or copy manually:\n"+
			"  copy \"%s\" \"%s\"",
		src, dst,
	)
}
