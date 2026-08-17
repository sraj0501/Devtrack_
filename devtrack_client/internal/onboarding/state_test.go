package onboarding

import (
	"os"
	"testing"
)

func TestResultRoundTripAndFirstSuccessWins(t *testing.T) {
	old := os.Getenv("XDG_DATA_HOME")
	t.Cleanup(func() { _ = os.Setenv("XDG_DATA_HOME", old) })
	if err := os.Setenv("XDG_DATA_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}

	if err := WriteResult(Result{CommitCount: 42, WordCount: 280, ProfilePath: "/tmp/profile.md"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteResult(Result{CommitCount: 99, WordCount: 999}); err != nil {
		t.Fatal(err)
	}

	got, err := ReadResult()
	if err != nil {
		t.Fatal(err)
	}
	if got.CommitCount != 42 || got.WordCount != 280 || got.ProfilePath != "/tmp/profile.md" || got.CompletedAt.IsZero() {
		t.Fatalf("unexpected result: %+v", got)
	}
}
