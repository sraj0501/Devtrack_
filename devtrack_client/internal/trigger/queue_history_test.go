package trigger

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueueHistoryContractAndUTCTimestamps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queue/history" || r.URL.Query().Get("action_type") != "eod_report" || r.URL.Query().Get("hours") != "0" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if r.Header.Get("X-DevTrack-API-Key") != "test-key" {
			t.Error("missing auth")
		}
		fmt.Fprint(w, `{"actions":[{"id":7,"action_type":"eod_report","payload":"{\"narrative\":\"captured work\"}","status":"rejected","created_at":"2026-10-21T12:00:00","expires_at":"2026-10-21T12:05:00","acted_at":"2026-10-21T12:01:00Z"}]}`)
	}))
	defer srv.Close()
	c := &HTTPTriggerClient{serverURL: srv.URL, apiKey: "test-key", httpClient: srv.Client()}
	actions, err := c.QueueHistory("eod_report", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].ID != 7 || actions[0].Status != "rejected" {
		t.Fatalf("%+v", actions)
	}
	if actions[0].CreatedAt.Location() != time.UTC || actions[0].ActedAt == nil {
		t.Fatal("timestamps lost UTC or acted_at")
	}
}

func TestQueueHistoryUnavailableIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	c := &HTTPTriggerClient{serverURL: srv.URL, httpClient: srv.Client()}
	if _, err := c.QueueHistory("eod_report", 0, 1); err == nil {
		t.Fatal("unavailable server must not look empty")
	}
}
