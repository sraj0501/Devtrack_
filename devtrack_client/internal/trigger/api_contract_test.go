package trigger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// TestAPIContract validates that the client's HTTP trigger machinery produces
// the correct outbound request shape as defined in docs/HTTP_API.md.
//
// These tests use an httptest.NewServer mock — they do NOT import any Python or
// server code. All assertions are on the Go side of the HTTP boundary only.
//
// TODO: extend with /trigger/commit shape test (TASK-045 or later)
// TODO: extend with /trigger/timer shape test
// TODO: extend with /trigger/boardroom shape test

// TestAPIContractHealth verifies that the client correctly hits GET /health
// and accepts a { "status": "ok" } response without error.
func TestAPIContractHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/health" {
			t.Errorf("expected path /health, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok","service":"devtrack-webhooks"}`)); err != nil {
			t.Errorf("mock server write error: %v", err)
		}
	}))
	defer srv.Close()

	// Build a client pointing at the mock server.
	// We bypass NewHTTPTriggerClient() to avoid reading env vars in tests.
	c := &HTTPTriggerClient{
		serverURL:  srv.URL,
		apiKey:     "",
		httpClient: &http.Client{},
	}

	ok := c.Ping()
	if !ok {
		t.Error("Ping() returned false — expected true when server returns HTTP 200")
	}
}

// TestAPIContractHealthShape verifies the /health response has the required
// JSON shape as per docs/HTTP_API.md: { "status": "ok", "service": "..." }.
func TestAPIContractHealthShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok","service":"devtrack-webhooks"}`)); err != nil {
			t.Errorf("mock server write error: %v", err)
		}
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health") //nolint:noctx
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("could not decode /health response: %v", err)
	}

	status, ok := body["status"].(string)
	if !ok {
		t.Error("health response missing 'status' field")
	} else if status != "ok" {
		t.Errorf("expected status='ok', got %q", status)
	}

	if _, ok := body["service"]; !ok {
		t.Error("health response missing 'service' field")
	}
}

// TestAPIContractPingRejectsNon200 verifies the client's Ping() treats any
// non-200 response as a failed health check, matching the contract spec.
func TestAPIContractPingRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := &HTTPTriggerClient{
		serverURL:  srv.URL,
		apiKey:     "",
		httpClient: &http.Client{},
	}

	if c.Ping() {
		t.Error("Ping() returned true for HTTP 503 — expected false")
	}
}

// TestAPIContractAPIKeyHeader verifies that when an API key is configured,
// the client sends it in the X-DevTrack-API-Key header on POST requests,
// as documented in docs/HTTP_API.md.
func TestAPIContractAPIKeyHeader(t *testing.T) {
	const wantKey = "test-contract-key"
	var gotKey string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-DevTrack-API-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok","pong":true}`)); err != nil {
			t.Errorf("mock server write error: %v", err)
		}
	}))
	defer srv.Close()

	c := &HTTPTriggerClient{
		serverURL:  srv.URL,
		apiKey:     wantKey,
		httpClient: &http.Client{},
	}

	if err := c.SendPing(); err != nil {
		t.Fatalf("SendPing() failed: %v", err)
	}

	if gotKey != wantKey {
		t.Errorf("X-DevTrack-API-Key: got %q, want %q", gotKey, wantKey)
	}
}

func TestAPIContractClientEventSyncIsAuthenticatedAndReplayKeyed(t *testing.T) {
	const wantKey = "event-sync-key"
	var received ClientEventSyncPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/trigger/client_events" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-DevTrack-API-Key"); got != wantKey {
			t.Errorf("API key = %q, want %q", got, wantKey)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","accepted":1}`))
	}))
	defer srv.Close()

	client := &HTTPTriggerClient{
		serverURL: srv.URL, apiKey: wantKey, httpClient: &http.Client{},
	}
	payload := ClientEventSyncPayload{
		ClientID: "client-1",
		Events: []ClientEvent{{
			EventID: "triggers:42", TableName: "triggers", SourceRowID: "42", Revision: 3,
			Payload: map[string]any{"ticket_id": "TASK-114"}, UpdatedAt: "2026-08-11 10:00:00",
		}},
	}
	response, err := client.SendClientEvents(payload)
	if err != nil {
		t.Fatalf("SendClientEvents: %v", err)
	}
	if response.Accepted != 1 || received.ClientID != "client-1" || received.Events[0].EventID != "triggers:42" {
		t.Fatalf("unexpected response/request: response=%#v received=%#v", response, received)
	}
}

func TestAPIContractExecuteStagedQueueActionCarriesFullLocalIdentity(t *testing.T) {
	const wantKey = "queue-key"
	var received QueueStagedAction
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/queue/execute_staged" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-DevTrack-API-Key"); got != wantKey {
			t.Errorf("API key = %q, want %q", got, wantKey)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"posted","error":""}`))
	}))
	defer srv.Close()

	client := &HTTPTriggerClient{serverURL: srv.URL, apiKey: wantKey, httpClient: &http.Client{}}
	action := db.PendingAction{
		ID: 42, ActionType: "post_comment", Target: "TASK-114", Platform: "github",
		Workspace: "devtrack", Payload: `{"comment":"ready"}`, Confidence: 0.95,
	}
	response, err := client.ExecuteStagedQueueAction(action)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "posted" || received.ID != 42 || received.Payload != action.Payload {
		t.Fatalf("unexpected response/request: response=%#v received=%#v", response, received)
	}
}

// TestAPIContractCommitPayloadShape verifies that SendCommitTrigger sends a
// JSON object with the fields defined in docs/HTTP_API.md § POST /trigger/commit.
func TestAPIContractCommitPayloadShape(t *testing.T) {
	var receivedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("could not decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok","actions":[],"commit_hash":"abc123"}`)); err != nil {
			t.Errorf("mock server write error: %v", err)
		}
	}))
	defer srv.Close()

	c := &HTTPTriggerClient{
		serverURL:  srv.URL,
		apiKey:     "",
		httpClient: &http.Client{},
	}

	err := c.SendCommitTrigger(CommitTriggerData{
		CommitHash:    "abc123def456",
		CommitMessage: "fix: resolve login timeout issue",
		Author:        "dev@example.com",
		Branch:        "fix/login-timeout",
		PMPlatform:    "github",
	})
	if err != nil {
		t.Fatalf("SendCommitTrigger() failed: %v", err)
	}

	// Verify required fields are present in the serialised payload.
	requiredFields := []string{"commit_hash", "commit_message", "author", "branch"}
	for _, field := range requiredFields {
		if _, ok := receivedBody[field]; !ok {
			t.Errorf("commit payload missing required field %q", field)
		}
	}

	if hash, _ := receivedBody["commit_hash"].(string); hash != "abc123def456" {
		t.Errorf("commit_hash: got %q, want %q", hash, "abc123def456")
	}
}
