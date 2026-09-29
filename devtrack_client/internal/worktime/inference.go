package worktime

import "time"

// Policy bounds passive work-time inference. ActivityGap is the largest gap
// that may join two observations, Padding accounts for work immediately around
// an observation, and MaxWindow caps a single inferred window.
type Policy struct {
	ActivityGap time.Duration
	Padding     time.Duration
	MaxWindow   time.Duration
}

// DefaultPolicy is deliberately conservative: local observations within 45
// minutes share a window, each edge receives at most 15 minutes, and one
// inferred window can never exceed eight hours.
func DefaultPolicy() Policy {
	return Policy{
		ActivityGap: 45 * time.Minute,
		Padding:     15 * time.Minute,
		MaxWindow:   8 * time.Hour,
	}
}

// Context identifies the local work bucket to which activity belongs.
type Context struct {
	TicketRef     string
	RepoPath      string
	WorkspaceName string
}

// Window is the clock-controlled input/output of the inference algorithm.
type Window struct {
	FirstActivity time.Time
	LastActivity  time.Time
	Context       Context
	EvidenceCount int
}

// CanExtend reports whether current activity belongs to an existing window.
func CanExtend(window Window, current time.Time, context Context, policy Policy) bool {
	policy = normalized(policy)
	if window.Context != context || current.Before(window.LastActivity) {
		return false
	}
	if localDate(window.FirstActivity) != localDate(current) {
		return false
	}
	if current.Sub(window.LastActivity) > policy.ActivityGap {
		return false
	}
	start, _ := Bounds(window.FirstActivity, window.LastActivity, policy)
	candidateEnd := current.Add(policy.Padding)
	return candidateEnd.Sub(start) <= policy.MaxWindow
}

// Bounds returns the inferred interval. It never crosses the local-day
// boundary of the first observation and never exceeds MaxWindow.
func Bounds(first, last time.Time, policy Policy) (time.Time, time.Time) {
	policy = normalized(policy)
	if last.Before(first) {
		last = first
	}
	loc := first.Location()
	dayStart := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, loc)
	nextDay := dayStart.AddDate(0, 0, 1)
	start := first.Add(-policy.Padding)
	if start.Before(dayStart) {
		start = dayStart
	}
	end := last.Add(policy.Padding)
	if end.After(nextDay) {
		end = nextDay
	}
	if end.After(start.Add(policy.MaxWindow)) {
		end = start.Add(policy.MaxWindow)
	}
	return start, end
}

// Minutes returns whole inferred minutes with no negative result.
func Minutes(first, last time.Time, policy Policy) int {
	start, end := Bounds(first, last, policy)
	minutes := int(end.Sub(start) / time.Minute)
	if minutes < 0 {
		return 0
	}
	return minutes
}

// Confidence increases only with repeated local evidence and remains below an
// explicit user's confidence of 1.0.
func Confidence(evidenceCount int) float64 {
	switch {
	case evidenceCount >= 3:
		return 0.9
	case evidenceCount == 2:
		return 0.75
	default:
		return 0.5
	}
}

func normalized(policy Policy) Policy {
	defaults := DefaultPolicy()
	if policy.ActivityGap <= 0 {
		policy.ActivityGap = defaults.ActivityGap
	}
	if policy.Padding < 0 {
		policy.Padding = 0
	}
	if policy.MaxWindow <= 0 {
		policy.MaxWindow = defaults.MaxWindow
	}
	return policy
}

func localDate(value time.Time) string {
	return value.Format("2006-01-02")
}
