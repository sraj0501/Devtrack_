package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// startTicketSyncLoop pushes the current local ticket cache to the Python
// server immediately (startup push — no API call), then runs a full
// sync + push every TICKET_SYNC_INTERVAL_HOURS.
//
// The startup push is fast because it only reads from the SQLite tables that
// were populated by previous sync runs.  The periodic full sync hits the PM
// APIs; failures are logged but never crash the daemon.
// startTicketSyncLoop pushes the current local ticket cache to the Python
// server immediately (startup push — no API call), then runs a full
// sync + push every TICKET_SYNC_INTERVAL_HOURS.
func (d *Daemon) startTicketSyncLoop() {
	if !config.GetTicketSyncOnStart() {
		return
	}
	if d.PushCachedFn == nil || d.FullSyncFn == nil {
		log.Printf("ticket-sync: sync callbacks not set — loop disabled")
		return
	}

	intervalHours := config.GetTicketSyncIntervalHours()

	go func() {
		// Startup push: read from local SQLite, push to Python (no API call).
		// Runs after a short delay so the Python server has time to start.
		time.Sleep(5 * time.Second)
		log.Printf("ticket-sync: startup push (local cache → server)")
		d.PushCachedFn()

		ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Printf("ticket-sync: periodic full sync (interval=%dh)", intervalHours)
				d.FullSyncFn()
			case <-d.ctx.Done():
				return
			}
		}
	}()

	log.Printf("✓ Ticket sync loop started (startup push + every %dh)", intervalHours)
}

// RouteInternalForceTrigger is the path for the daemon's internal force-trigger
// endpoint. Served on config.GetIPCHost():config.GetDevTrackServerHTTPPort() — localhost only.
const RouteInternalForceTrigger = "/internal/force-trigger"

// RouteInternalReloadConfig is the path for the daemon's internal config-reload
// endpoint. Served on config.GetIPCHost():config.GetDevTrackServerHTTPPort() — localhost only.
const RouteInternalReloadConfig = "/internal/reload-config"

// RouteInternalStats serves trigger throughput stats for the Python admin panel.
const RouteInternalStats = "/internal/stats"

// RouteInternalActiveSession serves the active work session for the Python server.
const RouteInternalActiveSession = "/internal/sessions/active"

// RouteDialecticQuery serves the inference query endpoint for the Python server's
// inject_style() Signal 3 path. Auth-gated by X-DevTrack-API-Key.
const RouteDialecticQuery = "/dialectic/query"

// startInternalHTTPServer starts a lightweight HTTP server on
// config.GetIPCHost():config.GetDevTrackServerHTTPPort() that exposes internal control
// endpoints not intended for external consumers.
//
// Currently registered routes:
//
//	POST /internal/force-trigger      — trigger an immediate scheduler run
//	POST /internal/reload-config      — reload .env + YAML config without restart
//	GET  /internal/stats              — trigger throughput stats for Python admin panel
//	GET  /internal/sessions/active    — active work session for Python server
//	GET  /dialectic/query             — FTS5 inference search for Python inject_style()
func (d *Daemon) startInternalHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc(RouteInternalForceTrigger, d.handleInternalForceTrigger)
	mux.HandleFunc(RouteInternalReloadConfig, d.handleInternalReloadConfig)
	mux.HandleFunc(RouteInternalStats, d.handleInternalStats)
	mux.HandleFunc(RouteInternalActiveSession, d.handleInternalActiveSession)
	mux.HandleFunc(RouteDialecticQuery, d.handleDialecticQuery)

	addr := fmt.Sprintf("%s:%d", config.GetIPCHost(), config.GetDevTrackServerHTTPPort())
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Internal HTTP server listening on http://%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Internal HTTP server error: %v", err)
		}
	}()
}

// handleInternalForceTrigger handles POST /internal/force-trigger.
// It calls ForceImmediate() on the scheduler, bypassing the normal timer interval.
//
// Internal endpoint reachable only on localhost — no auth required.
func (d *Daemon) handleInternalForceTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.monitor == nil || d.monitor.Scheduler() == nil {
		log.Println("force-trigger: monitor/scheduler not ready")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "scheduler not ready"})
		return
	}

	log.Println("Force trigger requested via HTTP internal endpoint")
	d.monitor.Scheduler().ForceImmediate()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
}

// handleInternalStats handles GET /internal/stats.
// Returns a JSON snapshot of today's trigger activity for the Python admin panel
// and server TUI. Opening a fresh DB connection per request is intentional —
// the handler is called infrequently and the DB file is always on local disk.
func (d *Daemon) handleInternalStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	database, err := db.NewDatabase()
	if err != nil {
		log.Printf("internal/stats: failed to open DB: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "db unavailable"})
		return
	}
	defer database.Close()

	stats := database.GetTriggerStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleInternalActiveSession handles GET /internal/sessions/active.
// Returns the active work session as JSON, or {"active": false} if none.
func (d *Daemon) handleInternalActiveSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	database, err := db.NewDatabase()
	if err != nil {
		log.Printf("internal/sessions/active: failed to open DB: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "db unavailable"})
		return
	}
	defer database.Close()

	session, err := database.GetActiveWorkSession()
	if err != nil {
		log.Printf("internal/sessions/active: query error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if session == nil {
		json.NewEncoder(w).Encode(map[string]bool{"active": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"active":         true,
		"id":             session.ID,
		"started_at":     session.StartedAt,
		"ticket_ref":     session.TicketRef,
		"workspace_name": session.WorkspaceName,
		"description":    session.Description,
	})
}

// handleInternalReloadConfig handles POST /internal/reload-config.
// It reloads the YAML config and workspace list without restarting the daemon.
// Mirrors the SIGHUP handler in daemon_unix.go so Windows callers get the same behaviour.
//
// Internal endpoint reachable only on localhost — no auth required.
func (d *Daemon) handleInternalReloadConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Reload-config requested via HTTP internal endpoint")

	reloaded := []string{}
	errs := []string{}

	if cfg, err := config.LoadConfig(); err == nil {
		d.config = cfg
		reloaded = append(reloaded, "config")
		log.Println("✓ Configuration reloaded")
	} else {
		log.Printf("Error reloading config: %v", err)
		errs = append(errs, fmt.Sprintf("config: %v", err))
	}

	if d.monitor != nil {
		d.monitor.ReloadWorkspaces()
		reloaded = append(reloaded, "workspaces")
		go func() {
			if err := trigger.NewHTTPTriggerClient().SendWorkspaceReload(); err != nil {
				log.Printf("Could not notify Python of workspace reload: %v", err)
			}
		}()
	}

	status := "ok"
	code := http.StatusOK
	if len(errs) > 0 {
		status = "partial"
		code = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"status":   status,
		"reloaded": reloaded,
		"errors":   errs,
	})
}

// handleDialecticQuery handles GET /dialectic/query.
// Auth: X-DevTrack-API-Key header (checked when DEVTRACK_API_KEY env var is set).
//
// Query params:
//
//	q            — FTS5 search query; if empty, returns inferences ordered by confidence DESC
//	context_type — optional filter by context type (commit, comment, report, etc.)
//	limit        — max results (default 5)
//
// Response: {"inferences": [{"id": N, "subject": "...", "inference": "...", "confidence": 0.85}, ...]}
func (d *Daemon) handleDialecticQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Auth gate — check API key when configured.
	configuredKey := os.Getenv("DEVTRACK_API_KEY")
	if configuredKey != "" {
		sentKey := r.Header.Get("X-DevTrack-API-Key")
		if sentKey != configuredKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
			return
		}
	}

	q := r.URL.Query().Get("q")
	contextType := r.URL.Query().Get("context_type")

	limit := 5
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if parsed, err := fmt.Sscanf(lStr, "%d", &limit); err != nil || parsed != 1 || limit < 1 {
			limit = 5
		}
	}

	database, err := db.NewDatabase()
	if err != nil {
		log.Printf("dialectic/query: failed to open DB: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "db unavailable"})
		return
	}
	defer database.Close()

	var inferences []db.Inference
	if q != "" {
		inferences, err = database.SearchInferences(q, limit)
	} else {
		inferences, err = database.ListInferencesByConfidence(contextType, limit)
	}
	if err != nil {
		log.Printf("dialectic/query: query error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Serialize to the response shape expected by InferenceRetriever.
	type inferenceJSON struct {
		ID         int64   `json:"id"`
		Subject    string  `json:"subject"`
		Inference  string  `json:"inference"`
		Confidence float64 `json:"confidence"`
	}
	out := make([]inferenceJSON, 0, len(inferences))
	for _, inf := range inferences {
		out = append(out, inferenceJSON{
			ID:         inf.ID,
			Subject:    inf.Subject,
			Inference:  inf.Inference,
			Confidence: inf.Confidence,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"inferences": out})
}

// startHeartbeatLoop sends a client heartbeat to the server on startup and
// every 60 seconds so the admin dashboard can show connected clients and their
// monitored workspaces.  Failures are logged but never crash the daemon.
func (d *Daemon) startHeartbeatLoop() {
	go func() {
		send := func() {
			wsCfg, err := config.LoadWorkspacesConfig()
			if err != nil {
				log.Printf("heartbeat: could not load workspaces: %v", err)
				return
			}
			hostname, _ := os.Hostname()
			workspaces := make([]trigger.HeartbeatWorkspace, 0, len(wsCfg.Workspaces))
			for _, ws := range wsCfg.GetEnabledWorkspaces() {
				workspaces = append(workspaces, trigger.HeartbeatWorkspace{
					Name:     ws.Name,
					Platform: ws.PMPlatform,
				})
			}
			payload := trigger.HeartbeatPayload{
				ClientID:   hostname,
				Version:    config.GetDevTrackVersion(),
				TLSEnabled: config.IsTLSEnabled(),
				Workspaces: workspaces,
			}
			if err := trigger.NewHTTPTriggerClient().SendHeartbeat(payload); err != nil {
				log.Printf("heartbeat: send failed: %v", err)
			}
		}

		// Send immediately (after a brief delay for the server to start).
		time.Sleep(8 * time.Second)
		send()

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				send()
			case <-d.ctx.Done():
				return
			}
		}
	}()
}

