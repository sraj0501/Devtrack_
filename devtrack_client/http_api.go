package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// RouteInternalForceTrigger is the path for the daemon's internal force-trigger
// endpoint. Served on GetIPCHost():GetDevTrackServerHTTPPort() — localhost only.
const RouteInternalForceTrigger = "/internal/force-trigger"

// RouteInternalReloadConfig is the path for the daemon's internal config-reload
// endpoint. Served on GetIPCHost():GetDevTrackServerHTTPPort() — localhost only.
const RouteInternalReloadConfig = "/internal/reload-config"

// startInternalHTTPServer starts a lightweight HTTP server on
// GetIPCHost():GetDevTrackServerHTTPPort() that exposes internal control
// endpoints not intended for external consumers.
//
// Currently registered routes:
//
//	POST /internal/force-trigger  — trigger an immediate scheduler run
//	POST /internal/reload-config  — reload .env + YAML config without restart
func (d *Daemon) startInternalHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc(RouteInternalForceTrigger, d.handleInternalForceTrigger)
	mux.HandleFunc(RouteInternalReloadConfig, d.handleInternalReloadConfig)

	addr := fmt.Sprintf("%s:%d", GetIPCHost(), GetDevTrackServerHTTPPort())
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

	if d.monitor == nil || d.monitor.scheduler == nil {
		log.Println("force-trigger: monitor/scheduler not ready")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "scheduler not ready"})
		return
	}

	log.Println("Force trigger requested via HTTP internal endpoint")
	d.monitor.scheduler.ForceImmediate()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
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

	if config, err := LoadConfig(); err == nil {
		d.config = config
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
			if err := NewHTTPTriggerClient().SendWorkspaceReload(); err != nil {
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
