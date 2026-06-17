package trigger

// TestHTTPTriggerClient_SendCommitTrigger_IsFirstCommitForTicket (TASK-073)
// confirms that when IsFirstCommitForTicket is true on CommitTriggerData the
// field is present in the JSON payload POSTed to the Python server, and that
// when it is false the field is omitted (omitempty).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPTriggerClient_SendCommitTrigger_IsFirstCommitTrue(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	_ = c.SendCommitTrigger(CommitTriggerData{
		CommitHash: "abc123",
		Branch:     "feat/PROJ-42-new-feature",
		TicketID:   "PROJ-42",
		IsFirstCommitForTicket: true,
	})

	val, present := body["is_first_commit_for_ticket"]
	if !present {
		t.Fatal("expected is_first_commit_for_ticket in payload when true, but it was absent")
	}
	boolVal, ok := val.(bool)
	if !ok {
		t.Fatalf("expected is_first_commit_for_ticket to be bool, got %T (%v)", val, val)
	}
	if !boolVal {
		t.Errorf("expected is_first_commit_for_ticket=true in payload, got false")
	}
}

func TestHTTPTriggerClient_SendCommitTrigger_IsFirstCommitFalse_Omitted(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	_ = c.SendCommitTrigger(CommitTriggerData{
		CommitHash:             "def456",
		Branch:                 "feat/PROJ-42-second-commit",
		TicketID:               "PROJ-42",
		IsFirstCommitForTicket: false, // zero value — omitempty should drop this
	})

	if _, present := body["is_first_commit_for_ticket"]; present {
		t.Error("expected is_first_commit_for_ticket to be omitted from payload when false (omitempty)")
	}
}
