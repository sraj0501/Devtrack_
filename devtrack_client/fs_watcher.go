package main

import "github.com/fsnotify/fsnotify"

// fsnotify op constants aliased for use in daemon.go without a direct import.
const (
	fsnotifyWrite  = fsnotify.Write
	fsnotifyCreate = fsnotify.Create
)

// newFsnotifyWatcher creates a new fsnotify.Watcher.
func newFsnotifyWatcher() (*fsnotify.Watcher, error) {
	return fsnotify.NewWatcher()
}
