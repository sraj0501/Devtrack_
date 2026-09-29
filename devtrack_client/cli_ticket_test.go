package main

import "testing"

func TestDefaultTicketKey(t *testing.T) {
	for _, tc := range []struct{ name, platform, want string }{
		{"project", "github", "GH"},
		{"project", "gitlab", "GL"},
		{"project", "azure", "ADO"},
		{"my-project", "jira", "MYPROJECT"},
		{"123", "none", "WORK"},
	} {
		if got := defaultTicketKey(tc.name, tc.platform); got != tc.want {
			t.Errorf("defaultTicketKey(%q, %q) = %q, want %q", tc.name, tc.platform, got, tc.want)
		}
	}
}
