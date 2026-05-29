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
	hash := "a689ae8f1234"
	msg := "feat(date): add parser\n\nLong body here."

	// Without time
	got := buildCommentBody(hash, msg, 0, false)
	want := "Commit a689ae8f: feat(date): add parser"
	if got != want {
		t.Errorf("buildCommentBody (no time) = %q, want %q", got, want)
	}

	// With time
	got = buildCommentBody(hash, msg, 90, true)
	want = "Commit a689ae8f: feat(date): add parser\n\nTime spent: 1h30m"
	if got != want {
		t.Errorf("buildCommentBody (with time) = %q, want %q", got, want)
	}
}
