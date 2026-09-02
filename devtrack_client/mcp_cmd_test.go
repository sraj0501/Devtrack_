package main

import (
	"path/filepath"
	"testing"
)

func TestMCPDatabasePath(t *testing.T) {
	relative := filepath.Join("testdata", "devtrack.db")
	want, err := filepath.Abs(relative)
	if err != nil {
		t.Fatal(err)
	}
	got, err := mcpDatabasePath([]string{"--database", relative})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("database path = %q, want %q", got, want)
	}
	if _, err := mcpDatabasePath([]string{"--database"}); err == nil {
		t.Fatal("missing --database value should fail")
	}
	if _, err := mcpDatabasePath([]string{"--unknown"}); err == nil {
		t.Fatal("unknown option should fail")
	}
}
