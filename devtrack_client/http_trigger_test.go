package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newMockServer starts a test HTTP server that records the last received
// request body and responds with statusCode / body.
func newMockServer(t *testing.T, statusCode int, responseBody string) (*httptest.Server, *[]byte) {
	t.Helper()
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			// store raw bytes if JSON decode fails
		}
		w.WriteHeader(statusCode)
		w.Write([]byte(responseBody))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// clientFor returns an HTTPTriggerClient pointed at the given server URL
// without TLS (uses plain http).
func clientFor(t *testing.T, serverURL, apiKey string) *HTTPTriggerClient {
	t.Helper()
	return &HTTPTriggerClient{
		serverURL:  serverURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// ---------------------------------------------------------------------------
// Ping
// ---------------------------------------------------------------------------

func TestHTTPTriggerClient_Ping_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	if !c.Ping() {
		t.Error("expected Ping to return true for a reachable server")
	}
}

func TestHTTPTriggerClient_Ping_ServerDown(t *testing.T) {
	// Use a URL that has no listener.
	c := clientFor(t, "http://127.0.0.1:19999", "")
	if c.Ping() {
		t.Error("expected Ping to return false when server is unreachable")
	}
}

func TestHTTPTriggerClient_Ping_EmptyURL(t *testing.T) {
	c := clientFor(t, "", "")
	if c.Ping() {
		t.Error("expected Ping to return false when serverURL is empty")
	}
}

func TestHTTPTriggerClient_Ping_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	if c.Ping() {
		t.Error("expected Ping to return false for non-200 status")
	}
}

// ---------------------------------------------------------------------------
// SendCommitTrigger
// ---------------------------------------------------------------------------

func TestHTTPTriggerClient_SendCommitTrigger_Success(t *testing.T) {
	srv, _ := newMockServer(t, 200, `{"status":"ok"}`)
	c := clientFor(t, srv.URL, "")

	err := c.SendCommitTrigger(CommitTriggerData{
		CommitHash:    "abc123",
		CommitMessage: "fix: test commit",
		Branch:        "main",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHTTPTriggerClient_SendCommitTrigger_ServerError(t *testing.T) {
	srv, _ := newMockServer(t, 500, `{"error":"internal"}`)
	c := clientFor(t, srv.URL, "")

	err := c.SendCommitTrigger(CommitTriggerData{CommitHash: "abc123"})
	if err == nil {
		t.Error("expected error for HTTP 500 response")
	}
}

func TestHTTPTriggerClient_SendCommitTrigger_MissingURL(t *testing.T) {
	c := clientFor(t, "", "")
	err := c.SendCommitTrigger(CommitTriggerData{CommitHash: "abc123"})
	if err == nil {
		t.Error("expected error when serverURL is empty")
	}
}

func TestHTTPTriggerClient_SendCommitTrigger_SetsAPIKeyHeader(t *testing.T) {
	var receivedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("X-DevTrack-API-Key")
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "my-secret-key")
	_ = c.SendCommitTrigger(CommitTriggerData{CommitHash: "abc123"})

	if receivedKey != "my-secret-key" {
		t.Errorf("expected API key header 'my-secret-key', got %q", receivedKey)
	}
}

func TestHTTPTriggerClient_SendCommitTrigger_NoAPIKeyHeaderWhenEmpty(t *testing.T) {
	var receivedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("X-DevTrack-API-Key")
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	_ = c.SendCommitTrigger(CommitTriggerData{CommitHash: "abc123"})

	if receivedKey != "" {
		t.Errorf("expected no API key header, got %q", receivedKey)
	}
}

func TestHTTPTriggerClient_SendCommitTrigger_PayloadJSON(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	_ = c.SendCommitTrigger(CommitTriggerData{
		CommitHash:    "deadbeef",
		CommitMessage: "feat: add trigger",
		Branch:        "feature/x",
	})

	if body["commit_hash"] != "deadbeef" {
		t.Errorf("payload commit_hash mismatch: %v", body["commit_hash"])
	}
	if body["branch"] != "feature/x" {
		t.Errorf("payload branch mismatch: %v", body["branch"])
	}
}

// ---------------------------------------------------------------------------
// SendTimerTrigger
// ---------------------------------------------------------------------------

func TestHTTPTriggerClient_SendTimerTrigger_Success(t *testing.T) {
	srv, _ := newMockServer(t, 200, `{"status":"accepted"}`)
	c := clientFor(t, srv.URL, "")

	err := c.SendTimerTrigger(TimerTriggerData{
		IntervalMins: 60,
		TriggerCount: 1,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHTTPTriggerClient_SendTimerTrigger_PayloadJSON(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	_ = c.SendTimerTrigger(TimerTriggerData{
		IntervalMins:  30,
		TriggerCount:  5,
		WorkspaceName: "my-project",
	})

	if body["interval_mins"] != float64(30) {
		t.Errorf("payload interval_mins mismatch: %v", body["interval_mins"])
	}
	if body["trigger_count"] != float64(5) {
		t.Errorf("payload trigger_count mismatch: %v", body["trigger_count"])
	}
}

func TestHTTPTriggerClient_SendTimerTrigger_ServerError(t *testing.T) {
	srv, _ := newMockServer(t, 503, `{}`)
	c := clientFor(t, srv.URL, "")

	err := c.SendTimerTrigger(TimerTriggerData{IntervalMins: 60})
	if err == nil {
		t.Error("expected error for HTTP 503 response")
	}
}

// ---------------------------------------------------------------------------
// SendWorkspaceReload
// ---------------------------------------------------------------------------

func TestHTTPTriggerClient_SendWorkspaceReload(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	err := c.SendWorkspaceReload()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if path != "/trigger/workspace_reload" {
		t.Errorf("expected path /trigger/workspace_reload, got %s", path)
	}
}

// ---------------------------------------------------------------------------
// SendPing
// ---------------------------------------------------------------------------

func TestHTTPTriggerClient_SendPing(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok","pong":true}`))
	}))
	defer srv.Close()

	c := clientFor(t, srv.URL, "")
	err := c.SendPing()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if path != "/trigger/ping" {
		t.Errorf("expected path /trigger/ping, got %s", path)
	}
}

// ---------------------------------------------------------------------------
// GetServerMode / GetServerURL (config helpers)
// ---------------------------------------------------------------------------

func TestGetServerMode_Defaults_Managed(t *testing.T) {
	os.Unsetenv("DEVTRACK_SERVER_MODE")
	// Ensure no cloud.json can be found (we're in test; cloud.go will return false)
	mode := GetServerMode()
	if mode != ServerModeManaged && mode != ServerModeCloud {
		t.Errorf("unexpected default server mode: %s", mode)
	}
}

func TestGetServerMode_External(t *testing.T) {
	os.Setenv("DEVTRACK_SERVER_MODE", "external")
	defer os.Unsetenv("DEVTRACK_SERVER_MODE")
	mode := GetServerMode()
	if mode != ServerModeExternal {
		t.Errorf("expected external, got %s", mode)
	}
}

func TestGetServerURL_UsesEnvVar(t *testing.T) {
	os.Setenv("DEVTRACK_SERVER_URL", "http://example.com:9000")
	os.Setenv("DEVTRACK_TLS", "false")
	defer func() {
		os.Unsetenv("DEVTRACK_SERVER_URL")
		os.Unsetenv("DEVTRACK_TLS")
	}()
	url := GetServerURL()
	if url != "http://example.com:9000" {
		t.Errorf("expected http://example.com:9000, got %s", url)
	}
}

func TestGetServerURL_DefaultsToLocalhost(t *testing.T) {
	os.Unsetenv("DEVTRACK_SERVER_URL")
	os.Setenv("DEVTRACK_TLS", "false")
	os.Setenv("WEBHOOK_PORT", "8089")
	defer func() {
		os.Unsetenv("DEVTRACK_TLS")
		os.Unsetenv("WEBHOOK_PORT")
	}()
	url := GetServerURL()
	if url != "http://127.0.0.1:8089" {
		t.Errorf("expected http://127.0.0.1:8089, got %s", url)
	}
}

func TestIsExternalServer_False_WhenManaged(t *testing.T) {
	os.Unsetenv("DEVTRACK_SERVER_MODE")
	// Without cloud.json this should be managed (not external)
	if IsExternalServer() {
		// Only fail if we're also not in cloud mode
		if GetServerMode() == ServerModeManaged {
			t.Error("expected IsExternalServer=false in managed mode")
		}
	}
}

func TestIsExternalServer_True_WhenExternal(t *testing.T) {
	os.Setenv("DEVTRACK_SERVER_MODE", "external")
	defer os.Unsetenv("DEVTRACK_SERVER_MODE")
	if !IsExternalServer() {
		t.Error("expected IsExternalServer=true in external mode")
	}
}
