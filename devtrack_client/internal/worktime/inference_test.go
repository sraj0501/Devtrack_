package worktime

import (
	"testing"
	"time"
)

func TestBoundsAndConfidenceAreClockControlled(t *testing.T) {
	loc := time.FixedZone("test", 2*60*60)
	first := time.Date(2026, 9, 29, 9, 0, 0, 0, loc)
	last := first.Add(30 * time.Minute)
	start, end := Bounds(first, last, DefaultPolicy())
	if want := first.Add(-15 * time.Minute); !start.Equal(want) {
		t.Fatalf("start = %s, want %s", start, want)
	}
	if want := last.Add(15 * time.Minute); !end.Equal(want) {
		t.Fatalf("end = %s, want %s", end, want)
	}
	if got := Minutes(first, last, DefaultPolicy()); got != 60 {
		t.Fatalf("minutes = %d, want 60", got)
	}
	if Confidence(1) != 0.5 || Confidence(2) != 0.75 || Confidence(3) != 0.9 {
		t.Fatal("unexpected confidence progression")
	}
}

func TestCanExtendSplitsGapsAndTicketChanges(t *testing.T) {
	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	context := Context{TicketRef: "TASK-159", RepoPath: "/repo", WorkspaceName: "devtrack"}
	window := Window{FirstActivity: base, LastActivity: base, Context: context, EvidenceCount: 1}
	if !CanExtend(window, base.Add(45*time.Minute), context, DefaultPolicy()) {
		t.Fatal("activity at the gap boundary should extend")
	}
	if CanExtend(window, base.Add(46*time.Minute), context, DefaultPolicy()) {
		t.Fatal("activity beyond the gap should start a new window")
	}
	changed := context
	changed.TicketRef = "TASK-160"
	if CanExtend(window, base.Add(5*time.Minute), changed, DefaultPolicy()) {
		t.Fatal("ticket changes must split inferred windows")
	}
}

func TestBoundsClampAtEODAndMaxWindow(t *testing.T) {
	late := time.Date(2026, 9, 29, 23, 55, 0, 0, time.UTC)
	_, end := Bounds(late, late, DefaultPolicy())
	if want := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC); !end.Equal(want) {
		t.Fatalf("EOD end = %s, want %s", end, want)
	}

	policy := DefaultPolicy()
	first := time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
	start, end := Bounds(first, first.Add(12*time.Hour), policy)
	if got := end.Sub(start); got != policy.MaxWindow {
		t.Fatalf("window = %s, want cap %s", got, policy.MaxWindow)
	}
}

func TestCanExtendRejectsCapAndDayBoundary(t *testing.T) {
	first := time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
	window := Window{FirstActivity: first, LastActivity: first.Add(7 * time.Hour)}
	if CanExtend(window, first.Add(7*time.Hour+45*time.Minute), Context{}, DefaultPolicy()) {
		t.Fatal("padding must count toward the maximum window")
	}
	window.FirstActivity = time.Date(2026, 9, 29, 23, 55, 0, 0, time.UTC)
	window.LastActivity = window.FirstActivity
	if CanExtend(window, window.FirstActivity.Add(10*time.Minute), Context{}, DefaultPolicy()) {
		t.Fatal("next-day activity must start a new window")
	}
}
