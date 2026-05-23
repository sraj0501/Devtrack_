package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// devtrackConfFile is the well-known file that records the path to the active .env.
// Lives at ~/.devtrack/devtrack.conf — one line: the absolute path to the .env file.
const devtrackConfFile = ".devtrack/devtrack.conf"

// AutoLoadEnv loads the registered .env file into the process environment.
// It is called once at the very start of main(), before any command processing.
//
// Resolution order (first match wins):
//  1. DEVTRACK_ENV_FILE env var (explicit override — CI, Docker, 1Password wrapper)
//  2. ~/.devtrack/devtrack.conf — path written by `devtrack setup`
//  3. .env in the same directory as the binary
//
// Existing env vars are NEVER overridden — shell exports, CI variables, and
// secret managers always take precedence over the file.
func AutoLoadEnv() {
	path := resolveEnvFilePath()
	if path == "" {
		return
	}
	_ = loadDotEnv(path) // silently skip if file unreadable
}

// resolveEnvFilePath returns the .env file path to load, or "" if none found.
func resolveEnvFilePath() string {
	// 1. Explicit override
	if p := os.Getenv("DEVTRACK_ENV_FILE"); p != "" {
		return p
	}

	// 2. ~/.devtrack/devtrack.conf
	if home, err := os.UserHomeDir(); err == nil {
		confPath := filepath.Join(home, devtrackConfFile)
		if data, err := os.ReadFile(confPath); err == nil {
			registered := strings.TrimSpace(string(data))
			if registered != "" {
				return registered
			}
		}
	}

	// 3. .env next to the binary
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// loadDotEnv parses a .env file and injects variables into the process environment.
// Variables already present in the environment are not overwritten.
// Supports: KEY=VALUE, KEY="quoted value", # comments, blank lines.
// Does NOT support variable expansion (${VAR}) — generated .env uses absolute paths.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Must contain '='
		idx := strings.IndexByte(line, '=')
		if idx < 1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip inline comments (value  # comment) — only when value is unquoted
		if !strings.HasPrefix(val, `"`) && !strings.HasPrefix(val, `'`) {
			if ci := strings.Index(val, " #"); ci >= 0 {
				val = strings.TrimSpace(val[:ci])
			}
		}

		// Strip surrounding quotes
		val = stripQuotes(val)

		// Key must be a valid identifier
		if !isValidEnvKey(key) {
			continue
		}

		// Only set if not already in environment (existing env wins)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

// stripQuotes removes a matching pair of " or ' surrounding a value.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// isValidEnvKey returns true if key contains only alphanumeric chars and underscores.
func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for _, c := range key {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// RegisterEnvFile writes the env file path to ~/.devtrack/devtrack.conf.
// Called by `devtrack setup` after writing the .env file.
func RegisterEnvFile(envPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	confDir := filepath.Join(home, ".devtrack")
	if err := os.MkdirAll(confDir, 0700); err != nil {
		return err
	}
	confPath := filepath.Join(confDir, "devtrack.conf")
	return os.WriteFile(confPath, []byte(envPath+"\n"), 0600)
}
