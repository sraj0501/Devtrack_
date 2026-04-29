package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Migration is a single idempotent upgrade step.
// Apply must be safe to skip if not needed (check before act).
type Migration struct {
	ID          string
	Description string
	Apply       func() error
}

type migrationState struct {
	SchemaVersion int               `json:"schema_version"`
	Applied       map[string]string `json:"applied"` // id → RFC3339 timestamp
}

// allMigrations is the ordered, append-only registry of all known migrations.
// Never reorder or remove entries — only append new ones at the end.
var allMigrations = []Migration{
	{
		ID:          "001-create-xdg-home",
		Description: "Create ~/.local/share/devtrack/ data directories",
		Apply: func() error {
			home, err := devtrackDataHome()
			if err != nil {
				return err
			}
			return createDataDirectories(filepath.Join(home, "data"))
		},
	},
	{
		ID:          "002-create-workspaces-yaml",
		Description: "Create workspaces.yaml in XDG home if missing",
		Apply: func() error {
			home, err := devtrackDataHome()
			if err != nil {
				return err
			}
			wsPath := filepath.Join(home, "workspaces.yaml")
			ws := os.Getenv("DEVTRACK_WORKSPACE")
			if ws == "" {
				ws = "."
			}
			pm := os.Getenv("PM_AGENT_DEFAULT_PLATFORM")
			if pm == "" {
				pm = "none"
			}
			return createWorkspacesFile(wsPath, ws, pm)
		},
	},
	{
		ID:          "003-add-workspaces-file-env",
		Description: "Add WORKSPACES_FILE= to .env if missing or empty",
		Apply: func() error {
			home, err := devtrackDataHome()
			if err != nil {
				return err
			}
			return patchEnvKey("WORKSPACES_FILE", filepath.Join(home, "workspaces.yaml"))
		},
	},
	{
		ID:          "004-generate-admin-secret-key",
		Description: "Generate ADMIN_SECRET_KEY in .env if empty",
		Apply: func() error {
			return patchEnvKey("ADMIN_SECRET_KEY", generateSecret(32))
		},
	},
}

// RunPendingMigrations applies any migrations that have not yet been recorded
// in the state file.  Prints one line per migration applied.
// Non-fatal: a migration failure prints a warning but does not stop the daemon.
func RunPendingMigrations() {
	state, err := loadMigrationState()
	if err != nil {
		// State file unreadable — start fresh (safe, all migrations are idempotent)
		state = &migrationState{SchemaVersion: 1, Applied: make(map[string]string)}
	}

	ran := 0
	for _, m := range allMigrations {
		if _, done := state.Applied[m.ID]; done {
			continue
		}
		if err := m.Apply(); err != nil {
			fmt.Printf("  [migrate] warning: %s skipped: %v\n", m.ID, err)
			continue
		}
		state.Applied[m.ID] = time.Now().UTC().Format(time.RFC3339)
		ran++
		fmt.Printf("  [migrate] applied: %s — %s\n", m.ID, m.Description)
	}

	if ran > 0 {
		if err := saveMigrationState(state); err != nil {
			fmt.Printf("  [migrate] warning: could not save migration state: %v\n", err)
		}
	}
}

// MarkAllMigrationsApplied records every known migration as applied without
// running them.  Called after a fresh `devtrack setup` so migrations don't
// re-run what setup already did.
func MarkAllMigrationsApplied() {
	state := &migrationState{SchemaVersion: 1, Applied: make(map[string]string)}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, m := range allMigrations {
		state.Applied[m.ID] = now
	}
	_ = saveMigrationState(state) // best-effort
}

// migrationStatePath returns ~/.devtrack/migrations.json.
// Uses the existing ~/.devtrack dir that RegisterEnvFile already creates.
func migrationStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devtrack", "migrations.json")
}

func loadMigrationState() (*migrationState, error) {
	data, err := os.ReadFile(migrationStatePath())
	if err != nil {
		return nil, err
	}
	var s migrationState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Applied == nil {
		s.Applied = make(map[string]string)
	}
	return &s, nil
}

func saveMigrationState(s *migrationState) error {
	path := migrationStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// patchEnvKey adds or replaces KEY=value in the registered .env file.
// If the key already has a non-empty value it is left untouched (idempotent).
func patchEnvKey(key, value string) error {
	envPath := resolveEnvFilePath()
	if envPath == "" {
		return nil // no .env registered yet — skip silently
	}

	raw, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("read .env: %w", err)
	}

	lines := strings.Split(string(raw), "\n")
	prefix := key + "="
	found := false

	for i, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		found = true
		existing := strings.TrimSpace(line[len(prefix):])
		// Strip inline comments
		if ci := strings.Index(existing, " #"); ci >= 0 {
			existing = strings.TrimSpace(existing[:ci])
		}
		existing = strings.Trim(existing, `"'`)
		if existing != "" {
			return nil // already has a value — leave it
		}
		lines[i] = prefix + value
		break
	}

	if !found {
		lines = append(lines, prefix+value)
	}

	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0600)
}
