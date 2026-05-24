package main

// Version information injected at build time via ldflags:
//
//	go build -ldflags="\
//	  -X main.Version=v1.2.3 \
//	  -X main.BuildTime=$(date -u +%FT%TZ) \
//	  -X main.GitCommit=$(git rev-parse --short HEAD)"
//
// GoReleaser sets these automatically from the git tag.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)
