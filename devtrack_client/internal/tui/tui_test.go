package tui

import (
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

func TestRenderHeaderUsesRuntimeModeAndBuildVersion(t *testing.T) {
	t.Setenv("DEVTRACK_SERVER_MODE", "external")
	config.SetBuildVersion("v9.8.7-test")
	t.Cleanup(func() { config.SetBuildVersion("dev") })

	header := (tuiModel{width: 80}).renderHeader()
	if !strings.Contains(header, "external") {
		t.Fatalf("header does not contain runtime mode: %q", header)
	}
	if !strings.Contains(header, "v9.8.7-test") {
		t.Fatalf("header does not contain build version: %q", header)
	}
}
