package ticket

import (
	"testing"
)

func TestDefaultExtractor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "jira branch PROJ-123",
			input:    "feat/PROJ-123-add-login",
			expected: "PROJ-123",
		},
		{
			name:     "ADO branch AB-7",
			input:    "fix/AB-7-button-color",
			expected: "AB-7",
		},
		{
			name:     "github issue ref #42 no leading hash",
			input:    "fix/#42-crash",
			expected: "42",
		},
		{
			name:     "lowercase prefix no match",
			input:    "feat/proj-44",
			expected: "",
		},
		{
			name:     "commit message with ticket",
			input:    "fix bug in login AB-99",
			expected: "AB-99",
		},
		{
			name:     "no ticket in message",
			input:    "chore: update readme",
			expected: "",
		},
		{
			name:     "branch with no ticket",
			input:    "feat/no-ticket-here",
			expected: "",
		},
	}

	ext := DefaultExtractor()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ext.Extract(tc.input)
			if got != tc.expected {
				t.Errorf("Extract(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestNewExtractor_CustomPattern(t *testing.T) {
	ext, err := NewExtractor(`(?P<ticket>DT-\d+)`)
	if err != nil {
		t.Fatalf("NewExtractor returned unexpected error: %v", err)
	}

	got := ext.Extract("feat/DT-999-thing")
	if got != "DT-999" {
		t.Errorf("Extract with custom pattern = %q, want %q", got, "DT-999")
	}
}

func TestNewExtractor_EmptyIsSameAsDefault(t *testing.T) {
	ext, err := NewExtractor("")
	if err != nil {
		t.Fatalf("NewExtractor(\"\") returned unexpected error: %v", err)
	}

	// Jira pattern should work.
	got := ext.Extract("feat/PROJ-123-add-login")
	if got != "PROJ-123" {
		t.Errorf("NewExtractor(\"\").Extract(...) = %q, want %q", got, "PROJ-123")
	}
}

func TestNewExtractor_BadRegex(t *testing.T) {
	_, err := NewExtractor(`[invalid`)
	if err == nil {
		t.Error("NewExtractor with bad regex should return non-nil error, got nil")
	}
}

func TestExtract_CommitMessageAB99(t *testing.T) {
	ext := DefaultExtractor()
	got := ext.Extract("fix bug in login AB-99")
	if got != "AB-99" {
		t.Errorf("Extract(\"fix bug in login AB-99\") = %q, want %q", got, "AB-99")
	}
}
