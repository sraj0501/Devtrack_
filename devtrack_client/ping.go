package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPingURL     = "https://ping.devtrack.dev"
	pingTimeout        = 5 * time.Second
	activePingInterval = 24 * time.Hour
	installIDFile      = ".devtrack/id"         // relative to home dir
	lastActivePingFile = "last_active_ping"      // relative to Data dir
	telemetryDisabled  = "telemetry_disabled"    // relative to Data dir
)

// pingPayload is what gets POSTed to the ping server.
type pingPayload struct {
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Event       string `json:"event"`
	Version     string `json:"version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
}

// isTelemetryDisabled returns true when the user has run `devtrack telemetry off`.
func isTelemetryDisabled() bool {
	dataDir := dataDir()
	if dataDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dataDir, telemetryDisabled))
	return err == nil
}

// dataDir returns the DevTrack data directory, empty string on any error.
// Avoids calling GetDevTrackDir() which panics on missing config.
func dataDir() string {
	d := os.Getenv("DEVTRACK_HOME")
	if d != "" {
		return d
	}
	root := os.Getenv("PROJECT_ROOT")
	if root != "" {
		return filepath.Join(root, "Data")
	}
	return ""
}

// getOrCreateInstallID reads ~/.devtrack/id, creating it (random UUID) if absent.
func getOrCreateInstallID() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return uuid.New().String()
	}
	idPath := filepath.Join(home, installIDFile)
	if data, err := os.ReadFile(idPath); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}
	id := uuid.New().String()
	_ = os.MkdirAll(filepath.Dir(idPath), 0700)
	_ = os.WriteFile(idPath, []byte(id+"\n"), 0600)
	return id
}

// hardwareFingerprint returns a stable, hashed hardware identifier.
// The raw value never leaves the machine — only the SHA-256 hex digest is sent.
func hardwareFingerprint() string {
	raw := rawMachineID()
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("sha256:%x", sum)
}

// rawMachineID fetches the platform-specific stable hardware ID.
func rawMachineID() string {
	switch runtime.GOOS {
	case "darwin":
		// IOPlatformUUID — stable across reboots, changes only on motherboard swap
		out, err := exec.Command(
			"ioreg", "-rd1", "-c", "IOPlatformExpertDevice",
		).Output()
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "IOPlatformUUID") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					return strings.Trim(strings.TrimSpace(parts[1]), `"`)
				}
			}
		}
	case "linux":
		// /etc/machine-id — set once on first boot, stable across reboots
		data, err := os.ReadFile("/etc/machine-id")
		if err == nil {
			return strings.TrimSpace(string(data))
		}
		// Fallback: dbus machine id
		data, err = os.ReadFile("/var/lib/dbus/machine-id")
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// shouldSendActivePing returns true if no active ping has been sent in the last 24 h.
func shouldSendActivePing() bool {
	d := dataDir()
	if d == "" {
		return false
	}
	path := filepath.Join(d, lastActivePingFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return true // file missing → first time
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return true
	}
	return time.Since(t) > activePingInterval
}

// markActivePingSent writes the current timestamp so the next run knows.
func markActivePingSent() {
	d := dataDir()
	if d == "" {
		return
	}
	_ = os.MkdirAll(d, 0755)
	_ = os.WriteFile(
		filepath.Join(d, lastActivePingFile),
		[]byte(time.Now().UTC().Format(time.RFC3339)+"\n"),
		0644,
	)
}

// pingURL returns the configured ping endpoint, empty string if disabled.
func pingURL() string {
	if u := os.Getenv("DEVTRACK_PING_URL"); u != "" {
		return u
	}
	return defaultPingURL
}

// sendPing fires an anonymous event ping in the background.
// It returns immediately — network errors are silently swallowed.
func sendPing(event string) {
	if isTelemetryDisabled() {
		return
	}
	url := pingURL()
	if url == "" {
		return
	}
	go func() {
		id := getOrCreateInstallID()
		fp := hardwareFingerprint()
		payload := pingPayload{
			ID:          id,
			Fingerprint: fp,
			Event:       event,
			Version:     Version,
			OS:          runtime.GOOS,
			Arch:        runtime.GOARCH,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return
		}
		client := &http.Client{Timeout: pingTimeout}
		resp, err := client.Post(url+"/ping", "application/json", bytes.NewReader(body))
		if err != nil {
			return
		}
		resp.Body.Close()
		if event == "active" {
			markActivePingSent()
		}
	}()
}

// SendActivePingIfDue fires an "active" ping at most once per 24 h.
// Called from handleStart (parent process only).
func SendActivePingIfDue() {
	if shouldSendActivePing() {
		sendPing("active")
	}
}
