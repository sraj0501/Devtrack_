package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

// Version is set at build time via -ldflags "-X main.Version=x.y.z"
var Version = "dev"

// HTTPAPIServer exposes the daemon over HTTP for devtrack-cli.
// Types here mirror contract/api.go — keep in sync when changing the API surface.
type HTTPAPIServer struct {
	daemon *Daemon
	server *http.Server
}

// response types — mirror contract/api.go exactly
type healthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

type statusResponse struct {
	Running    bool   `json:"running"`
	PID        int    `json:"pid,omitempty"`
	Uptime     string `json:"uptime,omitempty"`
	Monitoring string `json:"monitoring,omitempty"`
	Paused     bool   `json:"paused"`
}

type logsResponse struct {
	Lines []string `json:"lines"`
}

type commandResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type versionResponse struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func NewHTTPAPIServer(daemon *Daemon) *HTTPAPIServer {
	return &HTTPAPIServer{daemon: daemon}
}

func (h *HTTPAPIServer) Start() error {
	port := GetHTTPAPIPort()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.auth(h.handleHealth))
	mux.HandleFunc("/status", h.auth(h.handleStatus))
	mux.HandleFunc("/logs", h.auth(h.handleLogs))
	mux.HandleFunc("/version", h.auth(h.handleVersion))
	mux.HandleFunc("/start", h.auth(h.handleStart))
	mux.HandleFunc("/stop", h.auth(h.handleStop))
	mux.HandleFunc("/pause", h.auth(h.handlePause))
	mux.HandleFunc("/resume", h.auth(h.handleResume))
	mux.HandleFunc("/trigger/force", h.auth(h.handleForceTrigger))

	h.server = &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP API server error: %v", err)
		}
	}()

	log.Printf("HTTP API server listening on :%s", port)
	return nil
}

func (h *HTTPAPIServer) Stop() {
	if h.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.server.Shutdown(ctx); err != nil {
		log.Printf("HTTP API server shutdown error: %v", err)
	}
}

// auth checks X-DevTrack-Token when DEVTRACK_API_TOKEN is set.
func (h *HTTPAPIServer) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := GetAPIToken()
		if token != "" && r.Header.Get("X-DevTrack-Token") != token {
			w.WriteHeader(http.StatusUnauthorized)
			writeJSON(w, errorResponse{Error: "unauthorized", Code: http.StatusUnauthorized})
			return
		}
		next(w, r)
	}
}

func (h *HTTPAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, healthResponse{OK: true, Version: Version})
}

func (h *HTTPAPIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.daemon.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := statusResponse{
		Running: status.Running,
		PID:     status.PID,
		Paused:  h.daemon.monitor != nil && h.daemon.monitor.scheduler != nil && h.daemon.monitor.scheduler.IsPaused(),
	}
	if status.Running && status.Uptime > 0 {
		resp.Uptime = status.Uptime.Round(time.Second).String()
	}

	writeJSON(w, resp)
}

func (h *HTTPAPIServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	tail := 50
	if n := r.URL.Query().Get("tail"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 {
			tail = v
		}
	}

	lines, err := h.daemon.GetLogs(tail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read logs: %v", err))
		return
	}

	writeJSON(w, logsResponse{Lines: lines})
}

func (h *HTTPAPIServer) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, versionResponse{
		Version:   Version,
		GoVersion: runtime.Version(),
	})
}

// handleStart: HTTP server is only reachable when the daemon is running,
// so the daemon is already running by definition.
func (h *HTTPAPIServer) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	pid, _ := h.daemon.readPID()
	writeJSON(w, commandResponse{
		OK:      true,
		Message: fmt.Sprintf("daemon is already running (PID: %d)", pid),
	})
}

func (h *HTTPAPIServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if !h.daemon.IsRunning() {
		writeError(w, http.StatusBadRequest, "daemon is not running")
		return
	}
	writeJSON(w, commandResponse{OK: true, Message: "daemon stopping"})
	// Flush response before cancelling — cancel triggers daemon.Stop() via context
	go func() {
		time.Sleep(100 * time.Millisecond)
		h.daemon.cancel()
	}()
}

func (h *HTTPAPIServer) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if err := h.daemon.Pause(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, commandResponse{OK: true, Message: "scheduler paused"})
}

func (h *HTTPAPIServer) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if err := h.daemon.Resume(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, commandResponse{OK: true, Message: "scheduler resumed"})
}

func (h *HTTPAPIServer) handleForceTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if !h.daemon.IsRunning() {
		writeError(w, http.StatusBadRequest, "daemon is not running")
		return
	}
	if h.daemon.monitor == nil || h.daemon.monitor.scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not available")
		return
	}
	h.daemon.monitor.scheduler.ForceImmediate()
	writeJSON(w, commandResponse{OK: true, Message: "trigger fired"})
}

// helpers

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("HTTP API encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: msg, Code: code})
}

// GetAPIToken returns the optional API auth token (blank = no auth).
func GetAPIToken() string {
	return os.Getenv("DEVTRACK_API_TOKEN")
}
