package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ServerMode defines how the Python backend is managed
type ServerMode string

const (
	// ServerModeManaged — daemon spawns Python backend as subprocess (default for local use)
	ServerModeManaged ServerMode = "managed"
	// ServerModeExternal — Python backend runs independently; daemon does not spawn it
	ServerModeExternal ServerMode = "external"
	// ServerModeCloud — Python backend is a remote cloud-hosted server; credentials in ~/.devtrack/cloud.json
	ServerModeCloud ServerMode = "cloud"
	// ServerModeLightweight — git monitoring + scheduling only; no Python backend spawned
	ServerModeLightweight ServerMode = "lightweight"
)

// GetServerMode returns the configured server mode.
// Cloud credentials (~/.devtrack/cloud.json) take priority over env vars.
// Defaults to "managed" (spawn subprocess) if nothing is configured.
func GetServerMode() ServerMode {
	if IsCloudMode() {
		return ServerModeCloud
	}
	if os.Getenv("DEVTRACK_SERVER_MODE") == "lightweight" {
		return ServerModeLightweight
	}
	if os.Getenv("DEVTRACK_SERVER_MODE") == "external" {
		return ServerModeExternal
	}
	return ServerModeManaged
}

// GetServerURL returns the base URL of the Python backend server.
//
// Resolution order:
//  1. ~/.devtrack/cloud.json URL (when in cloud mode)
//  2. DEVTRACK_SERVER_URL env var (explicit override)
//  3. Managed mode default — https://127.0.0.1:<WEBHOOK_PORT>
func GetServerURL() string {
	if IsCloudMode() {
		if url := GetCloudURL(); url != "" {
			return url
		}
	}
	if v := os.Getenv("DEVTRACK_SERVER_URL"); v != "" {
		return v
	}
	port := os.Getenv("WEBHOOK_PORT")
	if port == "" {
		port = "8089"
	}
	scheme := "https"
	if !IsTLSEnabled() {
		scheme = "http"
	}
	return scheme + "://127.0.0.1:" + port
}

// IsTLSEnabled reports whether TLS is enabled for the Go↔Python HTTP channel.
// Defaults to true; set DEVTRACK_TLS=false to disable (dev / Docker environments).
func IsTLSEnabled() bool {
	v := os.Getenv("DEVTRACK_TLS")
	if v == "" {
		return true // on by default
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return true
	}
	return b
}

// GetTLSCertPath returns the path to the TLS server certificate PEM file.
// Uses DEVTRACK_TLS_CERT if set, otherwise Data/tls/server.crt under the database dir.
func GetTLSCertPath() string {
	if v := os.Getenv("DEVTRACK_TLS_CERT"); v != "" {
		return v
	}
	return filepath.Join(filepath.Dir(GetDatabaseDir()), "tls", "server.crt")
}

// GetTLSKeyPath returns the path to the TLS private key PEM file.
// Uses DEVTRACK_TLS_KEY if set, otherwise Data/tls/server.key under the database dir.
func GetTLSKeyPath() string {
	if v := os.Getenv("DEVTRACK_TLS_KEY"); v != "" {
		return v
	}
	return filepath.Join(filepath.Dir(GetDatabaseDir()), "tls", "server.key")
}

// IsExternalServer returns true when the Python backend is managed externally
// (i.e. the daemon should NOT spawn it as a subprocess).
// This includes Lightweight mode, where no Python backend is used at all.
func IsExternalServer() bool {
	mode := GetServerMode()
	return mode == ServerModeExternal || mode == ServerModeCloud || mode == ServerModeLightweight
}

// IsLightweightMode returns true when the daemon is running in Lightweight mode
// (git monitoring + scheduling only; Python backend is disabled).
func IsLightweightMode() bool {
	return GetServerMode() == ServerModeLightweight
}

// IsLocalTLS reports whether TLS cert-pinning (self-signed) should be used.
// True for managed/external-local mode; false for cloud mode where the remote
// server has a CA-signed cert and system roots are used instead.
func IsLocalTLS() bool {
	return IsTLSEnabled() && !IsCloudMode()
}

// RunInstall is called by `devtrack install`. It explains the client-server setup.
func RunInstall() error {
	fmt.Println("DevTrack uses a client-server architecture:")
	fmt.Println()
	fmt.Println("  Go binary (devtrack)  — client/daemon: git monitoring, scheduling, CLI")
	fmt.Println("  Python backend server — AI processing, integrations, reports")
	fmt.Println()
	fmt.Println("Setup options:")
	fmt.Println()
	fmt.Println("  LOCAL — managed (default)")
	fmt.Println("    DEVTRACK_SERVER_MODE=managed in .env")
	fmt.Println("    devtrack start           (daemon spawns Python automatically)")
	fmt.Println()
	fmt.Println("  LOCAL — external (separate process)")
	fmt.Println("    DEVTRACK_SERVER_MODE=external in .env")
	fmt.Println("    python python_bridge.py  (start Python server manually)")
	fmt.Println("    devtrack start           (then start the Go daemon)")
	fmt.Println()
	fmt.Println("  DOCKER")
	fmt.Println("    docker compose up        (starts Python server + infra)")
	fmt.Println()
	fmt.Println("  CLOUD")
	fmt.Println("    DEVTRACK_SERVER_URL=https://your-server.com in .env")
	fmt.Println("    DEVTRACK_SERVER_MODE=external")
	fmt.Println("    devtrack start")
	fmt.Println()
	fmt.Println("See docs/INSTALLATION.md for full setup instructions.")
	return nil
}
