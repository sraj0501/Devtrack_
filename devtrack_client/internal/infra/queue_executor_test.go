package infra

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

type fakeQueueTriggerClient struct {
	stagedResponse *trigger.QueueExecuteResponse
	stagedErr      error
	executed       chan db.PendingAction
	dialectic      chan struct{}
}

func (f *fakeQueueTriggerClient) GetQueuePending() (*trigger.QueuePendingResponse, error) {
	return &trigger.QueuePendingResponse{}, nil
}
func (f *fakeQueueTriggerClient) ExecuteQueueAction(int64) (*trigger.QueueExecuteResponse, error) {
	return &trigger.QueueExecuteResponse{Status: "posted"}, nil
}
func (f *fakeQueueTriggerClient) ExecuteStagedQueueAction(action db.PendingAction) (*trigger.QueueExecuteResponse, error) {
	if f.executed != nil {
		f.executed <- action
	}
	return f.stagedResponse, f.stagedErr
}
func (f *fakeQueueTriggerClient) SendClientEvents(trigger.ClientEventSyncPayload) (*trigger.ClientEventSyncResponse, error) {
	return &trigger.ClientEventSyncResponse{Status: "ok"}, nil
}
func (f *fakeQueueTriggerClient) PostDialecticInfer(db.PendingAction) ([]trigger.InferenceResult, error) {
	if f.dialectic != nil {
		f.dialectic <- struct{}{}
	}
	return nil, nil
}

func TestPendingActionFromServerPreservesNotificationFields(t *testing.T) {
	actedAt := "2026-08-11T10:05:00Z"
	actedBy := "cli"
	action := pendingActionFromServer(trigger.QueuePendingAction{
		ID: 7, ActionType: "post_comment", Target: "TASK-114",
		Platform: "github", Workspace: "devtrack", Payload: `{"comment":"ready"}`,
		Confidence: 0.75, Status: "pending",
		ExpiresAt: "2026-08-11T10:10:00Z", CreatedAt: "2026-08-11T10:00:00Z",
		ActedAt: &actedAt, ActedBy: &actedBy,
	})

	if action.ID != 7 || action.Payload != `{"comment":"ready"}` || action.Confidence != 0.75 {
		t.Fatalf("server action fields were lost: %#v", action)
	}
	if action.ExpiresAt.IsZero() || action.CreatedAt.IsZero() || action.ActedAt == nil {
		t.Fatalf("server action timestamps were not parsed: %#v", action)
	}
}

func TestMatchingLocalActionRejectsNumericIDCollision(t *testing.T) {
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("NewDatabaseAtPath: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	local := db.PendingAction{
		ActionType: "server_event_sync", Target: "client_events", Platform: "devtrack_server",
		Workspace: "local", Payload: `{"events":["local"]}`, Confidence: 1,
		Status: "pending", ExpiresAt: time.Now().Add(time.Minute),
	}
	id, err := database.InsertPendingAction(local)
	if err != nil {
		t.Fatalf("InsertPendingAction: %v", err)
	}

	executor := &QueueExecutor{db: database}
	remote := db.PendingAction{
		ID: id, ActionType: "post_comment", Target: "TASK-114", Platform: "github",
		Workspace: "devtrack", Payload: `{"comment":"remote"}`, Confidence: 0.95,
	}
	if got := executor.matchingLocalAction(remote); got != nil {
		t.Fatalf("numeric ID collision incorrectly matched local row: %#v", got)
	}

	stored, err := database.GetPendingAction(id)
	if err != nil || stored == nil || stored.ActionType != "server_event_sync" {
		t.Fatalf("local row changed while checking collision: row=%#v err=%v", stored, err)
	}
}

func TestMatchingLocalActionAcceptsExactMirror(t *testing.T) {
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("NewDatabaseAtPath: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	remote := db.PendingAction{
		ActionType: "post_comment", Target: "TASK-114", Platform: "github",
		Workspace: "devtrack", Payload: `{"comment":"same"}`, Confidence: 0.95,
		Status: "pending", ExpiresAt: time.Now().Add(time.Minute),
	}
	id, err := database.InsertPendingAction(remote)
	if err != nil {
		t.Fatalf("InsertPendingAction: %v", err)
	}
	remote.ID = id

	executor := &QueueExecutor{db: database}
	if got := executor.matchingLocalAction(remote); got == nil || got.ID != id {
		t.Fatalf("exact mirror did not match: %#v", got)
	}
}

func TestMaybeNotifyActionUsesServerPayloadOnce(t *testing.T) {
	received := make(chan db.PendingAction, 2)
	executor := &QueueExecutor{
		seenIDs: make(map[string]struct{}),
		NotifyFn: func(action db.PendingAction) {
			received <- action
		},
	}
	action := db.PendingAction{ID: 9, Confidence: 0.7, Payload: `{"comment":"visible"}`}
	executor.maybeNotifyAction("server", action)
	executor.maybeNotifyAction("server", action)

	select {
	case got := <-received:
		if got.Payload != action.Payload {
			t.Fatalf("notification payload = %q, want %q", got.Payload, action.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("notification callback was not called")
	}
	select {
	case <-received:
		t.Fatal("duplicate notification callback")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestTickLocalPendingActionsDispatchesExpiredAction(t *testing.T) {
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	id, err := database.InsertPendingAction(db.PendingAction{
		ActionType: "post_comment", Target: "TASK-114", Platform: "github",
		Workspace: "devtrack", Payload: `{"comment":"ready"}`, Confidence: 1,
		Status: "pending", ExpiresAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeQueueTriggerClient{
		stagedResponse: &trigger.QueueExecuteResponse{Status: "posted"},
		executed:       make(chan db.PendingAction, 1), dialectic: make(chan struct{}, 1),
	}
	executor := &QueueExecutor{db: database, triggerClient: fake, seenIDs: make(map[string]struct{})}
	executor.tickLocalPendingActions(time.Now())

	select {
	case action := <-fake.executed:
		if action.ID != id {
			t.Fatalf("executed action %d, want %d", action.ID, id)
		}
	default:
		t.Fatal("expired local action was not dispatched")
	}
	stored, err := database.GetPendingAction(id)
	if err != nil || stored == nil || stored.Status != "posted" {
		t.Fatalf("local action not marked posted: action=%#v err=%v", stored, err)
	}
	select {
	case <-fake.dialectic:
	case <-time.After(time.Second):
		t.Fatal("dialectic follow-up did not finish")
	}
}

func TestTickLocalPendingActionsRetainsTransportFailure(t *testing.T) {
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	id, err := database.InsertPendingAction(db.PendingAction{
		ActionType: "post_comment", Target: "TASK-114", Platform: "github",
		Workspace: "devtrack", Payload: `{}`, Confidence: 1,
		Status: "approved", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	executor := &QueueExecutor{
		db:            database,
		triggerClient: &fakeQueueTriggerClient{stagedErr: errors.New("offline")},
		seenIDs:       make(map[string]struct{}),
	}
	executor.tickLocalPendingActions(time.Now())

	stored, err := database.GetPendingAction(id)
	if err != nil || stored == nil || stored.Status != "approved" || stored.Error == nil {
		t.Fatalf("transport failure did not retain action: action=%#v err=%v", stored, err)
	}
}
