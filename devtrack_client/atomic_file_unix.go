//go:build !windows

package main

import "os"

func atomicReplaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
