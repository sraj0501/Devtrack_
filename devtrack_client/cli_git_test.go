package main

import "testing"

func TestParseMinutes(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantOK  bool
	}{
		{"", 0, false},
		{"   ", 0, false},
		{"skip", 0, false},
		{"30", 30, true},
		{"30m", 30, true},
		{"2h", 120, true},
		{"1h30m", 90, true},
		{"2h0m", 120, true},
		{"0", 0, false},   // zero is not a useful duration
		{"-5", 0, false},  // negative rejected
		{"1H30M", 90, true}, // case-insensitive
	}
	for _, c := range cases {
		got, ok := parseMinutes(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("parseMinutes(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestFormatMinutes(t *testing.T) {
	cases := map[int]string{
		45:  "45m",
		60:  "1h",
		90:  "1h30m",
		125: "2h5m",
	}
	for in, want := range cases {
		if got := formatMinutes(in); got != want {
			t.Errorf("formatMinutes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildCommentBody(t *testing.T) {
	hash := "a689ae8f1234ffff"
	msg := "feat(date): add parser"

	// Without time, default status — no Time spent / Status lines.
	got := buildCommentBody(hash, "Ada", msg, "in_progress", 0, false)
	want := "**Commit**: `a689ae8f1234`\n\n**Author**: Ada\n\n**Message**: feat(date): add parser"
	if got != want {
		t.Errorf("buildCommentBody (no time) =\n%q\nwant\n%q", got, want)
	}

	// With time and a done status.
	got = buildCommentBody(hash, "Ada", msg, "done", 90, true)
	want = "**Commit**: `a689ae8f1234`\n\n**Author**: Ada\n\n**Message**: feat(date): add parser\n\n**Time spent**: 1h30m\n\n**Status**: done"
	if got != want {
		t.Errorf("buildCommentBody (with time+status) =\n%q\nwant\n%q", got, want)
	}

	// Empty author omits the Author line.
	got = buildCommentBody(hash, "", msg, "in_progress", 0, false)
	if want := "**Commit**: `a689ae8f1234`\n\n**Message**: feat(date): add parser"; got != want {
		t.Errorf("buildCommentBody (no author) =\n%q\nwant\n%q", got, want)
	}
}

func TestParseTicketRefs(t *testing.T) {
	cases := map[string][]int{
		"fix login bug":             nil,
		"fixes #42":                 {42},
		"closes #7 and #13":         {7, 13},
		"AB#1234 work item":         {1234},
		"dup #5 #5":                 {5},
		"no number #":               nil,
	}
	for in, want := range cases {
		got := parseTicketRefs(in)
		if len(got) != len(want) {
			t.Errorf("parseTicketRefs(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parseTicketRefs(%q) = %v, want %v", in, got, want)
				break
			}
		}
	}
}

func TestDetectStatus(t *testing.T) {
	cases := map[string]string{
		"feat: add thing":     "in_progress",
		"fixes #42 login":     "done",
		"closes the redirect": "done",
		"resolve crash":       "done",
		"refactor module":     "in_progress",
	}
	for in, want := range cases {
		if got := detectStatus(in); got != want {
			t.Errorf("detectStatus(%q) = %q, want %q", in, got, want)
		}
	}
}
