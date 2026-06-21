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

// RouteDialecticPromoteSkill is the path for the daemon's skill promotion endpoint.
// Called by the Python SkillDetector when a pattern cluster crosses the emergence threshold.
const RouteDialecticPromoteSkill = "/dialectic/promote-skill"

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
//	POST /dialectic/promote-skill     — upsert a promoted skill from Python SkillDetector
func (d *Daemon) startInternalHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc(RouteInternalForceTrigger, d.handleInternalForceTrigger)
	mux.HandleFunc(RouteInternalReloadConfig, d.handleInternalReloadConfig)
	mux.HandleFunc(RouteInternalStats, d.handleInternalStats)
	mux.HandleFunc(RouteInternalActiveSession, d.handleInternalActiveSession)
	mux.HandleFunc(RouteDialecticPromoteSkill, d.handleDialecticPromoteSkill)

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

// handleDialecticPromoteSkill handles POST /dialectic/promote-skill.
// Called by the Python SkillDetector when a recurring inference cluster crosses
// the emergence threshold without developer corrections.
//
// Auth: X-DevTrack-API-Key header (same key as all /trigger/* endpoints).
// Body:  {"name":"...","description":"...","context_type":"...","evidence_count":N}
// Response 200: {"status":"promoted","skill_id":<int64>} on new insert,
//               {"status":"already_exists"} on upsert of an existing name.
func (d *Daemon) handleDialecticPromoteSkill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Auth: validate X-DevTrack-API-Key when DEVTRACK_API_KEY is configured.
	expected := os.Getenv("DEVTRACK_API_KEY")
	if expected != "" {
		key := r.Header.Get("X-DevTrack-API-Key")
		if key != expected {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	var body struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		ContextType   string `json:"context_type"`
		EvidenceCount int    `json:"evidence_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}
	if body.Name == "" || body.ContextType == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "name and context_type are required"})
		return
	}

	database, err := db.NewDatabase()
	if err != nil {
		log.Printf("promote-skill: failed to open DB: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "db unavailable"})
		return
	}
	defer database.Close()

	// Check if the skill already exists so we can return the correct status string.
	existing, err := database.GetSkill(body.Name)
	if err != nil {
		log.Printf("promote-skill: GetSkill error: %v", err)
	}

	if err := database.UpsertSkill(body.Name, body.Description, body.ContextType, body.EvidenceCount); err != nil {
		log.Printf("promote-skill: UpsertSkill error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if existing != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "already_exists"})
		return
	}
	// Fetch the newly inserted row to return its ID.
	skill, err := database.GetSkill(body.Name)
	if err != nil || skill == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "promoted"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"status": "promoted", "skill_id": skill.ID})
}

