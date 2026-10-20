package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/mcp"
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

func TestRegisterMCPToolsIncludesSage(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "devtrack.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	srv := mcp.New("test")
	if err := registerMCPTools(srv, database); err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"shutdown\"}\n")
	var output bytes.Buffer
	srv.RunOn(context.Background(), input, &output)
	text := output.String()
	for _, name := range []string{"get_active_context", "sage_search", "sage_topics", "sage_get_entry", "sage_status"} {
		if !strings.Contains(text, `"name":"`+name+`"`) {
			t.Errorf("tools/list omitted %s: %s", name, text)
		}
	}
}
