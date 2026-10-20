package db

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/worktime"
)

func TestWorkActivityRestartReplayAndCorrections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "time.db")
	database, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	activity := WorkActivity{SourceKind: "commit", SourceID: "first", OccurredAt: base, TicketRef: "TASK-159", RepoPath: "/repo", WorkspaceName: "test"}
	id, added, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy())
	if err != nil || !added {
		t.Fatalf("record: %v %v", added, err)
	}
	database.Close()
	database, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if got, added, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy()); err != nil || added || got != id {
		t.Fatalf("replay: %d %v %v", got, added, err)
	}
	activity.SourceID = "second"
	activity.OccurredAt = base.Add(30 * time.Minute)
	if got, _, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy()); err != nil || got != id {
		t.Fatalf("extend: %d %v", got, err)
	}
	if err := database.AdjustWorkSessionTime(id, 42); err != nil {
		t.Fatal(err)
	}
	if err := database.AdjustWorkSessionTime(id, 45); err != nil {
		t.Fatal(err)
	}
	sessions, err := database.GetWorkSessionsForDate("2026-09-29")
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions: %v %v", sessions, err)
	}
	if *sessions[0].DurationMinutes != 60 || *sessions[0].AdjustedMinutes != 45 || sessions[0].EvidenceCount != 2 {
		t.Fatalf("session: %+v", sessions[0])
	}
	history, err := database.ListWorkSessionAdjustments(id)
	if err != nil || len(history) != 2 || history[0].PreviousMinutes != nil || *history[1].PreviousMinutes != 42 {
		t.Fatalf("history: %+v %v", history, err)
	}
	activity.SourceID = "other-ticket"
	activity.TicketRef = "TASK-160"
	activity.OccurredAt = base.Add(40 * time.Minute)
	if got, _, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy()); err != nil || got == id {
		t.Fatalf("ticket split: %d %v", got, err)
	}
	sessions, err = database.GetWorkSessionsForDate("2026-09-29")
	if err != nil || len(sessions) != 2 {
		t.Fatalf("split sessions: %v %v", sessions, err)
	}
	previousEnd, _ := parseWorkSessionTime(*sessions[0].EndedAt)
	nextStart, _ := parseWorkSessionTime(sessions[1].StartedAt)
	if nextStart.Before(previousEnd) {
		t.Fatal("ticket switch double-counted padding")
	}
}

func TestStartOnlyAutomaticClosureDoesNotInventTime(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	id, err := database.InsertWorkSession("TASK-159", "/repo", "test")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	if _, err := database.ExecRaw("UPDATE work_sessions SET started_at=? WHERE id=?", base.Format(time.RFC3339), id); err != nil {
		t.Fatal(err)
	}
	if err := database.CloseInactiveWorkSession(base.Add(8*time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	sessions, err := database.GetWorkSessionsForDate("2026-09-29")
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions: %v %v", sessions, err)
	}
	if *sessions[0].DurationMinutes != 0 {
		t.Fatal("start-only session accrued idle time")
	}
}

func TestIdleAndEODClosureUseLastEvidence(t *testing.T) {
	for _, eod := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle", true: "EOD"}[eod], func(t *testing.T) {
			database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "time.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			id, err := database.InsertWorkSession("TASK-159", "/repo", "test")
			if err != nil {
				t.Fatal(err)
			}
			base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
			if _, err := database.ExecRaw("UPDATE work_sessions SET started_at=? WHERE id=?", base.Format(time.RFC3339), id); err != nil {
				t.Fatal(err)
			}
			_, _, err = database.RecordWorkActivity(WorkActivity{SourceKind: "commit", SourceID: "a", OccurredAt: base.Add(time.Hour), TicketRef: "TASK-159", RepoPath: "/repo", WorkspaceName: "test"}, worktime.DefaultPolicy())
			if err != nil {
				t.Fatal(err)
			}
			if err := database.CloseInactiveWorkSession(base.Add(70*time.Minute), 45*time.Minute); err != nil {
				t.Fatal(err)
			}
			active, err := database.GetActiveWorkSession()
			if err != nil || active == nil {
				t.Fatalf("recent activity closed: %v", err)
			}
			idle := 45 * time.Minute
			if eod {
				idle = 0
			}
			if err := database.CloseInactiveWorkSession(base.Add(8*time.Hour), idle); err != nil {
				t.Fatal(err)
			}
			sessions, err := database.GetWorkSessionsForDate("2026-09-29")
			if err != nil || len(sessions) != 1 {
				t.Fatalf("sessions: %v %v", sessions, err)
			}
			if *sessions[0].DurationMinutes != 60 || !sessions[0].AutoStopped {
				t.Fatalf("closure: %+v", sessions[0])
			}
		})
	}
}

func TestWorkSessionUpgradeFromLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	database, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.ExecRaw(`DROP TABLE work_sessions;
 CREATE TABLE work_sessions (id INTEGER PRIMARY KEY, started_at TEXT NOT NULL, ended_at TEXT,
 ticket_ref TEXT, repo_path TEXT, workspace_name TEXT, description TEXT, commits TEXT DEFAULT '[]',
 duration_minutes INTEGER, adjusted_minutes INTEGER, auto_stopped INTEGER DEFAULT 0,
 created_at TEXT DEFAULT (datetime('now')));`)
	if err != nil {
		t.Fatal(err)
	}
	database.Close()
	database, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatalf("legacy upgrade: %v", err)
	}
	database.Close()
}

func TestWorkTimeBoundaries(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		minutes []int
		tickets []string
	}{
		{"late then extend across future window", []int{60, 0, 30, 55, 80}, []string{"A", "A", "A", "A", "A"}},
		{"switch away and back", []int{0, 10, 20, 40, 65}, []string{"A", "B", "A", "A", "B"}},
		{"late conflicting tickets", []int{60, 0, 30, 45, 55, 70}, []string{"A", "B", "B", "A", "B", "A"}},
		{"same timestamp", []int{0, 0, 0, 15, 30}, []string{"A", "B", "A", "B", "A"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "time.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
			for i, minutes := range scenario.minutes {
				activity := WorkActivity{SourceKind: "commit", SourceID: fmt.Sprint(i), OccurredAt: base.Add(time.Duration(minutes) * time.Minute), TicketRef: scenario.tickets[i], RepoPath: "/repo", WorkspaceName: "test"}
				id, added, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy())
				if err != nil || !added {
					t.Fatalf("record %d: %v", i, err)
				}
				if replay, added, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy()); err != nil || added || replay != id {
					t.Fatalf("replay %d: %v", i, err)
				}
				assertNoWorkOverlap(t, database)
			}
		})
	}
}

func assertNoWorkOverlap(t *testing.T, database *Database) {
	t.Helper()
	sessions, err := database.GetWorkSessionsForDate("2026-09-29")
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range sessions {
		if a.EndedAt == nil {
			continue
		}
		start, err := parseWorkSessionTime(a.StartedAt)
		if err != nil {
			t.Fatal(err)
		}
		end, err := parseWorkSessionTime(*a.EndedAt)
		if err != nil {
			t.Fatal(err)
		}
		if end.Before(start) || a.DurationMinutes == nil || *a.DurationMinutes != int(end.Sub(start)/time.Minute) {
			t.Fatalf("invalid interval: %+v", a)
		}
		if end.Equal(start) {
			continue
		}
		for _, b := range sessions[i+1:] {
			if a.RepoPath != b.RepoPath || a.WorkspaceName != b.WorkspaceName {
				continue
			}
			otherStart, _ := parseWorkSessionTime(b.StartedAt)
			if b.EndedAt == nil {
				if end.After(otherStart) {
					t.Fatalf("completed %d overlaps active %d", a.ID, b.ID)
				}
				continue
			}
			otherEnd, _ := parseWorkSessionTime(*b.EndedAt)
			if otherEnd.After(otherStart) && start.Before(otherEnd) && otherStart.Before(end) {
				t.Fatalf("sessions %d and %d overlap", a.ID, b.ID)
			}
		}
	}
}

func TestExplicitStartTicketSwitchAndLateEvidence(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "time.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	record := func(hash, ticket string, minute int) int64 {
		t.Helper()
		id, _, err := database.RecordWorkActivity(WorkActivity{SourceKind: "commit", SourceID: hash, OccurredAt: base.Add(time.Duration(minute) * time.Minute), TicketRef: ticket, RepoPath: "/repo", WorkspaceName: "test"}, worktime.DefaultPolicy())
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	inferred := record("first", "A", 0)
	if err := database.AdjustWorkSessionTime(inferred, 40); err != nil {
		t.Fatal(err)
	}
	explicit, err := database.insertWorkSessionAt("A", "/repo", "test", base.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.insertWorkSessionAt("A", "/repo", "test", base.Add(6*time.Minute)); err == nil {
		t.Fatal("allowed overlapping explicit start")
	}
	if got := record("matching", "A", 20); got != explicit {
		t.Fatal("matching commit did not attach")
	}
	record("late-different", "B", 10)
	active, err := database.GetActiveWorkSession()
	if err != nil || active == nil || active.ID != explicit {
		t.Fatal("late ticket conflict closed active session")
	}
	record("switch", "B", 30)
	active, err = database.GetActiveWorkSession()
	if err != nil || active != nil {
		t.Fatal("ticket switch left explicit session running")
	}
	record("back", "A", 40)
	assertNoWorkOverlap(t, database)
	sessions, err := database.GetWorkSessionsForDate("2026-09-29")
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range sessions {
		if session.ID == inferred && (session.AdjustedMinutes == nil || *session.AdjustedMinutes != 40) {
			t.Fatal("start erased correction")
		}
		if session.ID == explicit && (session.DurationMinutes == nil || *session.DurationMinutes != 15) {
			t.Fatalf("explicit time included another ticket: %+v", session)
		}
	}
}

func TestCorrectionsSurviveLaterEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "time.db")
	database, err := NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	activity := WorkActivity{SourceKind: "commit", SourceID: "first", OccurredAt: base, TicketRef: "A", RepoPath: "/repo", WorkspaceName: "test"}
	id, _, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AdjustWorkSessionTime(id, 0); err != nil {
		t.Fatal(err)
	}
	if err := database.AdjustWorkSessionTime(id, -1); err == nil {
		t.Fatal("accepted negative correction")
	}
	if err := database.AdjustWorkSessionTime(id+100, 12); err == nil {
		t.Fatal("accepted missing session")
	}
	activity.SourceID = "second"
	activity.OccurredAt = base.Add(30 * time.Minute)
	if _, _, err := database.RecordWorkActivity(activity, worktime.DefaultPolicy()); err != nil {
		t.Fatal(err)
	}
	database.Close()
	database, err = NewDatabaseAtPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	sessions, err := database.GetWorkSessionsForDate("2026-09-29")
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions: %v", err)
	}
	if *sessions[0].DurationMinutes != 60 || sessions[0].AdjustedMinutes == nil || *sessions[0].AdjustedMinutes != 0 {
		t.Fatalf("correction lost: %+v", sessions[0])
	}
	history, err := database.ListWorkSessionAdjustments(id)
	if err != nil || len(history) != 1 || history[0].AdjustedMinutes != 0 || history[0].MeasuredMinutes == nil || *history[0].MeasuredMinutes != 30 {
		t.Fatalf("audit: %+v %v", history, err)
	}
}

func TestWorkWindowArrivalPermutations(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "time.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	order := []int{0, 1, 2, 3}
	run := 0
	var permute func(int)
	permute = func(n int) {
		if n != len(order) {
			for i := n; i < len(order); i++ {
				order[n], order[i] = order[i], order[n]
				permute(n + 1)
				order[n], order[i] = order[i], order[n]
			}
			return
		}
		run++
		for _, value := range order {
			_, _, err := database.RecordWorkActivity(WorkActivity{SourceKind: "commit", SourceID: fmt.Sprintf("%d-%d", run, value), OccurredAt: base.Add(time.Duration(value*20) * time.Minute), TicketRef: fmt.Sprint(value % 2), RepoPath: fmt.Sprintf("/repo%d", run), WorkspaceName: "test"}, worktime.DefaultPolicy())
			if err != nil {
				t.Fatal(err)
			}
			assertNoWorkOverlap(t, database)
		}
	}
	permute(0)
	if run != 24 {
		t.Fatalf("only %d permutations", run)
	}
}

func TestStopCannotRewriteAutomaticTicketClosure(t *testing.T) {
	database, err := NewDatabaseAtPath(filepath.Join(t.TempDir(), "time.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	id, err := database.insertWorkSessionAt("A", "/repo", "test", base)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.EndWorkSession(id, base.Add(-time.Minute).Format(time.RFC3339), 0); err == nil {
		t.Fatal("accepted stop before start")
	}
	if err := database.CloseInactiveWorkSession(base.Add(time.Hour), 0); err != nil {
		t.Fatal(err)
	}
	if err := database.EndWorkSession(id, base.Add(time.Hour).Format(time.RFC3339), 60); err == nil {
		t.Fatal("stale stop rewrote closure")
	}
}
