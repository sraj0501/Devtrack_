package infra

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
)

func TestSageDaemonWorkerPauseAndInflightCancellation(t *testing.T) {
	d, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	root := t.TempDir()
	if _, err := sage.SetPaused(root, true); err != nil {
		t.Fatal(err)
	}
	_, err = d.InsertSageEvent(sage.Event{SchemaVersion: 1, EventID: "one", Harness: "codex", SessionID: "s", EventType: "command", Tool: "shell", OccurredAt: time.Now(), Command: "git status", Signature: "git status"})
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	t.Setenv("DEVTRACK_SAGE_IDLE_POLL_MS", "50")
	im := &IntegratedMonitor{database: d}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := im.startSageDistillation(ctx, root, llmclient.Config{Host: server.URL, Model: "local", Client: server.Client()})
	select {
	case <-entered:
		t.Fatal("model called while paused")
	case <-time.After(150 * time.Millisecond):
	}
	if _, err := sage.SetPaused(root, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start request")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker failed to cancel in-flight model call")
	}
	var state string
	if err := d.DB().QueryRow(`SELECT state FROM sage_jobs`).Scan(&state); err != nil || state != "processing" {
		t.Fatalf("cancelled claim must remain recoverable: %s %v", state, err)
	}
}

func TestSageDaemonSpoolToDurableDraftSurvivesRestart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", home)
	t.Setenv("DEVTRACK_SAGE_IDLE_POLL_MS", "50")
	t.Setenv("DEVTRACK_SAGE_RETRY_DELAY_SECS", "1")
	root := filepath.Join(home, "devtrack", "sage")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			http.Error(w, "SECRET_CANARY outage", http.StatusServiceUnavailable)
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": "invalid model output"}, "done": true})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": `{"worth_documenting":true,"title":"Inspect state","section":"Git","commands":["git status"],"what":"Shows changes.","why":"Review before committing.","example":"git status","notes":""}`}, "done": true})
		}
	}))
	defer server.Close()
	t.Setenv("OLLAMA_HOST", server.URL)
	event := sage.Event{SchemaVersion: 1, EventID: "spooled", Harness: "codex", SessionID: "s", EventType: "command", Tool: "shell", OccurredAt: time.Now(), Command: "git status", Signature: "git status"}
	if err := sage.WriteEvent(root, event); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sage.db")
	for round := 0; round < 2; round++ {
		d, err := db.NewDatabaseAtPath(path)
		if err != nil {
			t.Fatal(err)
		}
		im := &IntegratedMonitor{database: d}
		im.startSageCapture(context.Background())
		if im.sageCancel == nil {
			d.Close()
			t.Fatal("Sage lifecycle did not start")
		}
		func() {
			defer d.Close()
			defer func() { im.sageCancel(); <-im.sageDone }()
			deadline := time.Now().Add(8 * time.Second)
			for {
				var state, draft string
				var attempts int
				err := d.DB().QueryRow(`SELECT state,draft_json,attempts FROM sage_jobs`).Scan(&state, &draft, &attempts)
				if err == nil && state == "distilled" {
					if attempts != 3 || draft == "" {
						t.Fatalf("attempts=%d draft=%q", attempts, draft)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("no durable draft: state=%s err=%v", state, err)
				}
				time.Sleep(20 * time.Millisecond)
			}
			// Give a restarted worker time to attempt a duplicate claim.
			if round == 1 {
				time.Sleep(150 * time.Millisecond)
			}
		}()
	}
	if calls.Load() != 3 {
		t.Fatalf("restart repeated model work: calls=%d", calls.Load())
	}
}
