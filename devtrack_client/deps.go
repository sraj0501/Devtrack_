package main

// This file ensures critical dependencies are included in go.mod
// Remove this file once these packages are actively imported in the codebase

import (
	_ "github.com/fsnotify/fsnotify"
	_ "github.com/go-git/go-git/v5"
	_ "github.com/robfig/cron/v3"
	_ "gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)
