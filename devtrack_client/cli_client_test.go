package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cli "gitlab.com/devtrack3_cloud/devtrack_cli"
	"gitlab.com/devtrack3_cloud/devtrack_contract"
)

// newTestServer creates a test HTTP server that serves a fixed JSON response
// and records the last request for inspection.
type testServer struct {
	*httptest.Server
	lastReq *http.Request
}

func newServer(handler http.HandlerFunc) *testServer {
	ts := &testServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.lastReq = r
		handler(w, r)
	}))
	return ts
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestHealth(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contract.RouteHealth {
			t.Errorf("path = %s, want %s", r.URL.Path, contract.RouteHealth)
		}
		writeJSON(w, contract.HealthResponse{OK: true, Version: "1.2.3"})
	})
	defer srv.Close()

	resp, err := cli.NewCLIClient(srv.URL, "").Health()
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Error("OK = false, want true")
	}
	if resp.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", resp.Version, "1.2.3")
	}
}

func TestStatus(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, contract.StatusResponse{Running: true, PID: 42, Uptime: "5m0s"})
	})
	defer srv.Close()

	resp, err := cli.NewCLIClient(srv.URL, "").Status()
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Running {
		t.Error("Running = false, want true")
	}
	if resp.PID != 42 {
		t.Errorf("PID = %d, want 42", resp.PID)
	}
	if resp.Uptime != "5m0s" {
		t.Errorf("Uptime = %q, want %q", resp.Uptime, "5m0s")
	}
}

func TestLogs_DefaultTail(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contract.RouteLogs {
			t.Errorf("path = %s, want %s", r.URL.Path, contract.RouteLogs)
		}
		writeJSON(w, contract.LogsResponse{Lines: []string{"line1", "line2"}})
	})
	defer srv.Close()

	resp, err := cli.NewCLIClient(srv.URL, "").Logs(50)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Lines) != 2 {
		t.Errorf("len(Lines) = %d, want 2", len(resp.Lines))
	}
	if tail := srv.lastReq.URL.Query().Get("tail"); tail != "50" {
		t.Errorf("tail query param = %q, want %q", tail, "50")
	}
}

func TestLogs_CustomTail(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, contract.LogsResponse{Lines: nil})
	})
	defer srv.Close()

	if _, err := cli.NewCLIClient(srv.URL, "").Logs(25); err != nil {
		t.Fatal(err)
	}
	if tail := srv.lastReq.URL.Query().Get("tail"); tail != "25" {
		t.Errorf("tail = %q, want %q", tail, "25")
	}
}

func TestStart(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != contract.RouteStart {
			t.Errorf("path = %s, want %s", r.URL.Path, contract.RouteStart)
		}
		writeJSON(w, contract.CommandResponse{OK: true, Message: "daemon running"})
	})
	defer srv.Close()

	resp, err := cli.NewCLIClient(srv.URL, "").Start()
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Error("OK = false, want true")
	}
}

func TestStop(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		writeJSON(w, contract.CommandResponse{OK: true, Message: "stopping"})
	})
	defer srv.Close()

	if _, err := cli.NewCLIClient(srv.URL, "").Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestPause(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, contract.CommandResponse{OK: true, Message: "paused"})
	})
	defer srv.Close()

	if _, err := cli.NewCLIClient(srv.URL, "").Pause(); err != nil {
		t.Fatal(err)
	}
}

func TestResume(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, contract.CommandResponse{OK: true, Message: "resumed"})
	})
	defer srv.Close()

	if _, err := cli.NewCLIClient(srv.URL, "").Resume(); err != nil {
		t.Fatal(err)
	}
}

func TestForceTrigger(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contract.RouteForceTrigger {
			t.Errorf("path = %s, want %s", r.URL.Path, contract.RouteForceTrigger)
		}
		writeJSON(w, contract.CommandResponse{OK: true, Message: "triggered"})
	})
	defer srv.Close()

	if _, err := cli.NewCLIClient(srv.URL, "").ForceTrigger(); err != nil {
		t.Fatal(err)
	}
}

func TestVersion(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, contract.VersionResponse{Version: "0.2.0", GoVersion: "go1.24"})
	})
	defer srv.Close()

	resp, err := cli.NewCLIClient(srv.URL, "").Version()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Version != "0.2.0" {
		t.Errorf("Version = %q, want %q", resp.Version, "0.2.0")
	}
}

func TestAuth_TokenSent(t *testing.T) {
	const secret = "my-token"
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, contract.HealthResponse{OK: true})
	})
	defer srv.Close()

	if _, err := cli.NewCLIClient(srv.URL, secret).Health(); err != nil {
		t.Fatal(err)
	}
	got := srv.lastReq.Header.Get(contract.AuthHeader)
	if got != secret {
		t.Errorf("auth header = %q, want %q", got, secret)
	}
}

func TestAuth_NoTokenWhenEmpty(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, contract.HealthResponse{OK: true})
	})
	defer srv.Close()

	if _, err := cli.NewCLIClient(srv.URL, "").Health(); err != nil {
		t.Fatal(err)
	}
	if got := srv.lastReq.Header.Get(contract.AuthHeader); got != "" {
		t.Errorf("auth header = %q, want empty", got)
	}
}

func TestErrorResponse_UsesErrorField(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(contract.ErrorResponse{Error: "unauthorized", Code: 401})
	})
	defer srv.Close()

	_, err := cli.NewCLIClient(srv.URL, "").Health()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "unauthorized" {
		t.Errorf("err = %q, want %q", err.Error(), "unauthorized")
	}
}

func TestErrorResponse_FallbackToStatusCode(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not json"))
	})
	defer srv.Close()

	_, err := cli.NewCLIClient(srv.URL, "").Health()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "server returned 503" {
		t.Errorf("err = %q, want %q", err.Error(), "server returned 503")
	}
}

func TestUnreachable(t *testing.T) {
	// Port 1 is reserved and always refuses connections
	_, err := cli.NewCLIClient("http://127.0.0.1:1", "").Health()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(err.Error()) < len("server unreachable:") {
		t.Errorf("err = %q, want prefix %q", err.Error(), "server unreachable:")
	}
}
