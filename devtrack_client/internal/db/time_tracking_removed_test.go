package db

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUpgradeRetiresQueuedWorkSessionSync(t *testing.T) {
	database := newTestDB(t)
	if err := database.initServerEventSync(); err != nil {
		t.Fatal(err)
	}
	legacy := ServerEvent{EventID: "work_sessions:1", TableName: "work_sessions", SourceRowID: "1", Revision: 1, Payload: map[string]any{"duration_minutes": 45}}
	bytes, err := json.Marshal(ServerEventSyncPayload{ClientID: "test", Events: []ServerEvent{legacy}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := database.InsertPendingAction(PendingAction{
		ActionType: ServerEventSyncActionType, Payload: string(bytes), Status: "approved",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.db.Exec(`INSERT INTO server_event_outbox(event_id, table_name, source_row_id, payload) VALUES ('work_sessions:1','work_sessions','1','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.db.Exec(`CREATE TRIGGER sync_work_sessions_insert AFTER INSERT ON work_sessions BEGIN INSERT INTO server_event_outbox(event_id,table_name,source_row_id,payload) VALUES ('work_sessions:legacy','work_sessions','legacy','{}'); END`); err != nil {
		t.Fatal(err)
	}
	if err := database.initServerEventSync(); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := database.db.QueryRow(`SELECT status FROM pending_actions WHERE id=?`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "rejected" {
		t.Fatalf("retired sync action remains %s", status)
	}
	if _, err := database.db.Exec(`INSERT INTO work_sessions(started_at,ticket_ref) VALUES ('2026-10-21T12:00:00Z','TEST-1')`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.db.QueryRow(`SELECT count(*) FROM server_event_outbox WHERE table_name='work_sessions'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("retired session data can still be synchronized: %d", count)
	}
	if _, err := DecodeServerEventSyncAction(PendingAction{ActionType: ServerEventSyncActionType, Payload: string(bytes)}); err == nil {
		t.Fatal("accepted a legacy duration-only batch")
	}
}

func TestStagedMixedBatchOmitsLegacyTimeData(t *testing.T) {
	bytes, err := json.Marshal(ServerEventSyncPayload{ClientID: "test", Events: []ServerEvent{
		{EventID: "work_sessions:1", TableName: "work_sessions", Payload: map[string]any{"duration_minutes": 45}},
		{EventID: "triggers:1", TableName: "triggers", Payload: map[string]any{"commit_hash": "abc"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := DecodeServerEventSyncAction(PendingAction{ActionType: ServerEventSyncActionType, Payload: string(bytes)})
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 1 || payload.Events[0].TableName != "triggers" {
		t.Fatalf("unexpected outgoing events: %+v", payload.Events)
	}
}
