//go:build !windows

package knowledge

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
