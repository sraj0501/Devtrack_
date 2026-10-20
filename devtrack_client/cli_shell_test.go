package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureShellInit(t *testing.T, render func() error) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = original }()

	if err := render(); err != nil {
		w.Close()
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestShellInitDoesNotInterceptNormalGitCommit(t *testing.T) {
	cli := &CLI{}
	tests := []struct {
		name   string
		render func() error
	}{
		{name: "bash", render: cli.handleShellInitBash},
		{name: "powershell", render: cli.handleShellInitPowerShell},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := captureShellInit(t, tc.render)
			for _, forbidden := range []string{"commit|history", "'commit','history", "devtrack git commit"} {
				if strings.Contains(out, forbidden) {
					t.Fatalf("shell integration still intercepts commit via %q", forbidden)
				}
			}
		})
	}
}
