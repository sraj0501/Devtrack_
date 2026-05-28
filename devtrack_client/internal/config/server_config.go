package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

// ServerMode defines how the Python backend is managed.
type ServerMode string

const (
	// ServerModeManaged — daemon spawns Python backend as subprocess (default).
	// Python must be installed locally. URL defaults to localhost.
	ServerModeManaged ServerMode = "managed"
	// ServerModeExternal — Python backend is managed outside the Go daemon.
	// This covers all external cases: same machine separate process, different
	// machine on the LAN, or a remote cloud server. Set DEVTRACK_SERVER_URL to
	// point at the server. Credentials can be stored via `devtrack cloud login`.
	// If DEVTRACK_SERVER_URL is unset, AI-requiring triggers are skipped silently.
	ServerModeExternal ServerMode = "external"
)

// GetServerMode returns the configured server mode.
// Cloud credentials (~/.devtrack/cloud.json) map to external mode — the stored
// URL is used as DEVTRACK_SERVER_URL. Defaults to managed if nothing is configured.
func GetServerMode() ServerMode {
	// Cloud credentials present → external mode (URL comes from cloud.json)
	if IsCloudMode() {
		return ServerModeExternal
	}
	if os.Getenv("DEVTRACK_SERVER_MODE") == "external" ||
		os.Getenv("DEVTRACK_SERVER_MODE") == "lightweight" {
		return ServerModeExternal
	}
	return ServerModeManaged
}

// GetServerURL returns the base URL of the Python backend server.
//
// Resolution order:
//  1. ~/.devtrack/cloud.json URL (when cloud credentials are stored)
//  2. DEVTRACK_SERVER_URL env var (explicit override)
//  3. Managed mode default — https://127.0.0.1:<WEBHOOK_PORT>
//
// Returns empty string in external mode with no URL configured.
func GetServerURL() string {
	// Cloud credentials take priority
	if u := GetCloudURL(); u != "" {
		return u
	}
	if v := os.Getenv("DEVTRACK_SERVER_URL"); v != "" {
		return v
	}
	if GetServerMode() == ServerModeExternal {
		// External with no URL — triggers will be skipped
		return ""
	}
	// Managed: default to localhost
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
		return true
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

// IsExternalServer returns true when the daemon should NOT spawn a Python subprocess.
func IsExternalServer() bool {
	return GetServerMode() == ServerModeExternal
}

// IsLocalTLS reports whether TLS cert-pinning (self-signed cert) should be used.
// True when the server URL resolves to localhost — managed mode and local external processes.
// False for remote URLs (different machine or cloud), which use system CA roots.
func IsLocalTLS() bool {
	if !IsTLSEnabled() {
		return false
	}
	serverURL := GetServerURL()
	if serverURL == "" {
		return false
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return false
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	return host == "localhost"
}

// RunInstall is called by `devtrack install`. It explains the client-server setup.
func RunInstall() error {
	fmt.Println("DevTrack uses a client-server architecture:")
	fmt.Println()
	fmt.Println("  Go binary (devtrack)  — client/daemon: git monitoring, scheduling, CLI, git-sage")
	fmt.Println("  Python backend server — AI processing, integrations, reports, boardroom")
	fmt.Println()
	fmt.Println("Two modes:")
	fmt.Println()
	fmt.Println("  MANAGED (default)")
	fmt.Println("    DEVTRACK_SERVER_MODE=managed  (or unset)")
	fmt.Println("    devtrack start                  daemon spawns the Python server automatically")
	fmt.Println("    Python must be installed on this machine.")
	fmt.Println()
	fmt.Println("  EXTERNAL")
	fmt.Println("    DEVTRACK_SERVER_MODE=external")
	fmt.Println("    DEVTRACK_SERVER_URL=https://<host>:<port>")
	fmt.Println("    DEVTRACK_API_KEY=<key>")
	fmt.Println("    devtrack start                  daemon connects to the external server")
	fmt.Println()
	fmt.Println("  The external server can be:")
	fmt.Println("    • a separate process on this machine  (DEVTRACK_SERVER_URL=https://localhost:8089)")
	fmt.Println("    • a server on your LAN                (DEVTRACK_SERVER_URL=https://192.168.1.x:8089)")
	fmt.Println("    • a remote cloud server               (use: devtrack cloud login --url URL --key KEY)")
	fmt.Println()
	fmt.Println("  If DEVTRACK_SERVER_URL is not set in external mode,")
	fmt.Println("  AI features are unavailable but git monitoring and scheduling still run.")
	fmt.Println()
	fmt.Println("See docs/INSTALLATION.md for full setup instructions.")
	return nil
}
