package db

import (
	"testing"
	"time"
)

func TestServerEventOutboxStagesAndReplaysLatestState(t *testing.T) {
	database := newTestDB(t)
	if err := database.initServerEventSync(); err != nil {
		t.Fatalf("initServerEventSync: %v", err)
	}

	triggerID, err := database.InsertTrigger(TriggerRecord{
		TriggerType:   "commit",
		Timestamp:     time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC),
		Source:        "git",
		RepoPath:      "/repo",
		CommitHash:    "abc123",
		CommitMessage: "fix: sync event",
		TicketID:      "TASK-114",
	})
	if err != nil {
		t.Fatalf("InsertTrigger: %v", err)
	}

	events, err := database.ListPendingServerEvents(100)
	if err != nil {
		t.Fatalf("ListPendingServerEvents: %v", err)
	}
	if len(events) != 1 || events[0].EventID != "triggers:1" {
		t.Fatalf("unexpected outbox events: %#v", events)
	}
	if got := events[0].Payload["ticket_id"]; got != "TASK-114" {
		t.Fatalf("ticket_id = %#v, want TASK-114", got)
	}

	now := time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC)
	action, err := database.StageServerEventSync(100, now)
	if err != nil {
		t.Fatalf("StageServerEventSync: %v", err)
	}
	if action == nil || action.ActionType != ServerEventSyncActionType {
		t.Fatalf("unexpected sync action: %#v", action)
	}
	payload, err := DecodeServerEventSyncAction(*action)
	if err != nil {
		t.Fatalf("DecodeServerEventSyncAction: %v", err)
	}
	if payload.ClientID == "" || len(payload.Events) != 1 {
		t.Fatalf("unexpected staged payload: %#v", payload)
	}

	if err := database.MarkServerEventsSynced(ServerEventKeys(payload)); err != nil {
		t.Fatalf("MarkServerEventsSynced: %v", err)
	}
	if err := database.MarkTriggerProcessed(triggerID); err != nil {
		t.Fatalf("MarkTriggerProcessed: %v", err)
	}
	events, err = database.ListPendingServerEvents(100)
	if err != nil {
		t.Fatalf("ListPendingServerEvents after update: %v", err)
	}
	if len(events) != 1 || events[0].Payload["processed"] != float64(1) {
		t.Fatalf("updated trigger was not re-queued with latest state: %#v", events)
	}
	if events[0].Revision != 2 {
		t.Fatalf("updated trigger revision = %d, want 2", events[0].Revision)
	}
}

func TestServerEventOutboxTracksAllApprovedCoreTables(t *testing.T) {
	database := newTestDB(t)
	if err := database.initServerEventSync(); err != nil {
		t.Fatalf("initServerEventSync: %v", err)
	}

	if _, err := database.InsertTaskUpdate(TaskUpdateRecord{
		Timestamp: time.Now().UTC(), Project: "DevTrack", TicketID: "TASK-114",
	}); err != nil {
		t.Fatalf("InsertTaskUpdate: %v", err)
	}
	if _, err := database.InsertWorkSession("TASK-114", "/repo", "devtrack"); err != nil {
		t.Fatalf("InsertWorkSession: %v", err)
	}
	if _, err := database.InsertPendingAction(PendingAction{
		ActionType: "post_comment", Target: "TASK-114", Platform: "github",
		Workspace: "devtrack", Payload: `{}`, Confidence: 0.9, Status: "pending",
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("InsertPendingAction: %v", err)
	}

	events, err := database.ListPendingServerEvents(100)
	if err != nil {
		t.Fatalf("ListPendingServerEvents: %v", err)
	}
	want := map[string]bool{
		"task_updates:1": false, "work_sessions:1": false, "pending_actions:1": false,
	}
	for _, event := range events {
		if _, ok := want[event.EventID]; ok {
			want[event.EventID] = true
		}
	}
	for eventID, found := range want {
		if !found {
			t.Errorf("missing outbox event %s in %#v", eventID, events)
		}
	}
}

func TestServerEventSyncClientIDIsStableAndMetaActionIsExcluded(t *testing.T) {
	database := newTestDB(t)
	if err := database.initServerEventSync(); err != nil {
		t.Fatalf("initServerEventSync: %v", err)
	}
	first, err := database.GetOrCreateServerEventClientID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := database.GetOrCreateServerEventClientID()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("client ID is not stable: %q != %q", first, second)
	}

	if _, err := database.InsertPendingAction(PendingAction{
		ActionType: ServerEventSyncActionType, Target: "server", Platform: "devtrack_server",
		Workspace: "all", Payload: `{}`, Confidence: 1, Status: "pending",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	events, err := database.ListPendingServerEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("sync metadata action recursively entered outbox: %#v", events)
	}
}

func TestServerEventAckCannotClearNewerRevision(t *testing.T) {
	database := newTestDB(t)
	if err := database.initServerEventSync(); err != nil {
		t.Fatal(err)
	}
	id, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now().UTC(), Source: "git",
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := database.ListPendingServerEvents(1)
	if err != nil || len(events) != 1 {
		t.Fatalf("initial outbox: events=%#v err=%v", events, err)
	}
	staleKey := ServerEventKey{EventID: events[0].EventID, Revision: events[0].Revision}
	if err := database.MarkTriggerProcessed(id); err != nil {
		t.Fatal(err)
	}
	if err := database.MarkServerEventsSynced([]ServerEventKey{staleKey}); err != nil {
		t.Fatal(err)
	}
	events, err = database.ListPendingServerEvents(1)
	if err != nil || len(events) != 1 || events[0].Revision != staleKey.Revision+1 {
		t.Fatalf("stale ack cleared newer event: events=%#v err=%v", events, err)
	}
}

func TestRejectServerEventSyncDoesNotRestageSameRevision(t *testing.T) {
	database := newTestDB(t)
	if err := database.initServerEventSync(); err != nil {
		t.Fatal(err)
	}
	id, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now().UTC(), Source: "git",
	})
	if err != nil {
		t.Fatal(err)
	}
	action, err := database.StageServerEventSync(10, time.Now().UTC())
	if err != nil || action == nil {
		t.Fatalf("stage: action=%#v err=%v", action, err)
	}
	if err := database.UpdatePendingActionStatus(action.ID, "rejected", "test"); err != nil {
		t.Fatal(err)
	}
	if next, err := database.StageServerEventSync(10, time.Now().UTC()); err != nil || next != nil {
		t.Fatalf("rejected revision was restaged: action=%#v err=%v", next, err)
	}

	if err := database.MarkTriggerProcessed(id); err != nil {
		t.Fatal(err)
	}
	if next, err := database.StageServerEventSync(10, time.Now().UTC()); err != nil || next == nil {
		t.Fatalf("new revision was not staged: action=%#v err=%v", next, err)
	}
}

func TestServerEventSyncPayloadCannotBeEdited(t *testing.T) {
	database := newTestDB(t)
	if err := database.initServerEventSync(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.InsertTrigger(TriggerRecord{
		TriggerType: "commit", Timestamp: time.Now().UTC(), Source: "git",
	}); err != nil {
		t.Fatal(err)
	}
	action, err := database.StageServerEventSync(10, time.Now().UTC())
	if err != nil || action == nil {
		t.Fatalf("stage: action=%#v err=%v", action, err)
	}
	if err := database.UpdatePendingActionPayload(action.ID, `{}`); err == nil {
		t.Fatal("server event sync payload edit unexpectedly succeeded")
	}
}
