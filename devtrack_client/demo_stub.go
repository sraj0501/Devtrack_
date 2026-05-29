//go:build !dev

package main

import "fmt"

// RunDemo is a no-op in production builds.
// Build with -tags dev to enable demo/test commands.
func RunDemo() {
	fmt.Println("Demo mode not available in production builds.")
	fmt.Println("Rebuild with: go build -tags dev .")
}
