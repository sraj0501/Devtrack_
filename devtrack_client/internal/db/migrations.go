package db

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
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
	{
		ID:          "005-prune-legacy-health-services",
		Description: "Remove stale MongoDB/Redis/IPC health snapshots (legacy Python-era services no longer checked by the Go daemon)",
		Apply: func() error {
			db, err := NewDatabase()
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			_, err = db.db.Exec(`
				DELETE FROM health_snapshots
				WHERE service IN ('redis', 'mongodb', 'mongo', 'python_bridge', 'ipc')
			`)
			return err
		},
	},
	{
		ID:          "006-create-pending-actions",
		Description: "Create pending_actions table and indexes for the Phase 1 approval queue",
		Apply: func() error {
			database, err := NewDatabase()
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()
			_, err = database.db.Exec(`
				CREATE TABLE IF NOT EXISTS pending_actions (
					id          INTEGER PRIMARY KEY AUTOINCREMENT,
					action_type TEXT    NOT NULL,
					target      TEXT    NOT NULL,
					platform    TEXT    NOT NULL,
					workspace   TEXT    NOT NULL,
					payload     TEXT    NOT NULL,
					confidence  REAL    NOT NULL,
					status      TEXT    NOT NULL DEFAULT 'pending',
					expires_at  DATETIME NOT NULL,
					created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
					acted_at    DATETIME,
					acted_by    TEXT,
					error       TEXT
				);
				CREATE INDEX IF NOT EXISTS idx_pending_actions_status ON pending_actions(status);
				CREATE INDEX IF NOT EXISTS idx_pending_actions_expires ON pending_actions(expires_at);
			`)
			return err
		},
	},
	{
		ID:          "007-add-ticket-id-to-triggers",
		Description: "Add ticket_id column to triggers table for Phase 2 ticket extraction",
		Apply: func() error {
			database, err := NewDatabase()
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()

			var count int
			if err := database.db.QueryRow(
				`SELECT COUNT(*) FROM pragma_table_info('triggers') WHERE name='ticket_id'`,
			).Scan(&count); err != nil {
				return fmt.Errorf("check ticket_id column: %w", err)
			}
			if count > 0 {
				return nil // already present — idempotent no-op
			}

			_, err = database.db.Exec(`ALTER TABLE triggers ADD COLUMN ticket_id TEXT DEFAULT ''`)
			return err
		},
	},
	{
		ID:          "008-create-inferences-fts5",
		Description: "Create inferences table, FTS5 virtual table, and sync triggers for Phase 6 dialectic self-improvement",
		Apply: func() error {
			database, err := NewDatabase()
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()

			_, err = database.db.Exec(`
				CREATE TABLE IF NOT EXISTS inferences (
					id           INTEGER PRIMARY KEY AUTOINCREMENT,
					context_type TEXT    NOT NULL,
					subject      TEXT    NOT NULL,
					inference    TEXT    NOT NULL,
					evidence     TEXT    NOT NULL,
					confidence   REAL    NOT NULL DEFAULT 0.5,
					source       TEXT    NOT NULL DEFAULT 'hermes3',
					created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
					updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
				);
			`)
			if err != nil {
				return fmt.Errorf("create inferences table: %w", err)
			}

			// Check if FTS5 virtual table already exists before creating it.
			// Use sqlite_master check for safe cross-version idempotency.
			var ftsCount int
			if err := database.db.QueryRow(
				`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='inferences_fts'`,
			).Scan(&ftsCount); err != nil {
				return fmt.Errorf("check inferences_fts: %w", err)
			}
			if ftsCount == 0 {
				_, err = database.db.Exec(`
					CREATE VIRTUAL TABLE inferences_fts USING fts5(
						context_type,
						subject,
						inference,
						content='inferences',
						content_rowid='id'
					);
				`)
				if err != nil {
					return fmt.Errorf("create inferences_fts virtual table: %w", err)
				}
			}

			// Create sync triggers for FTS5 (idempotent via IF NOT EXISTS).
			for _, trigDDL := range []string{
				`CREATE TRIGGER IF NOT EXISTS inferences_ai AFTER INSERT ON inferences BEGIN
					INSERT INTO inferences_fts(rowid, context_type, subject, inference)
					VALUES (new.id, new.context_type, new.subject, new.inference);
				END;`,
				`CREATE TRIGGER IF NOT EXISTS inferences_au AFTER UPDATE ON inferences BEGIN
					INSERT INTO inferences_fts(inferences_fts, rowid, context_type, subject, inference)
					VALUES('delete', old.id, old.context_type, old.subject, old.inference);
					INSERT INTO inferences_fts(rowid, context_type, subject, inference)
					VALUES (new.id, new.context_type, new.subject, new.inference);
				END;`,
				`CREATE TRIGGER IF NOT EXISTS inferences_ad AFTER DELETE ON inferences BEGIN
					INSERT INTO inferences_fts(inferences_fts, rowid, context_type, subject, inference)
					VALUES('delete', old.id, old.context_type, old.subject, old.inference);
				END;`,
			} {
				if _, err := database.db.Exec(trigDDL); err != nil {
					return fmt.Errorf("create inferences trigger: %w", err)
				}
			}

			return nil
		},
	},
	{
		ID:          "009-create-corrections",
		Description: "Create corrections table for Phase 6 developer feedback on inferences",
		Apply: func() error {
			database, err := NewDatabase()
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()

			_, err = database.db.Exec(`
				CREATE TABLE IF NOT EXISTS corrections (
					id            INTEGER PRIMARY KEY AUTOINCREMENT,
					inference_id  INTEGER NOT NULL REFERENCES inferences(id),
					correction    TEXT    NOT NULL,
					flagged_from  TEXT    NOT NULL DEFAULT 'tui',
					weight        REAL    NOT NULL DEFAULT 2.0,
					created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
				);
				CREATE INDEX IF NOT EXISTS idx_corrections_inference ON corrections(inference_id);
			`)
			return err
		},
	},
	{
		ID:          "010-create-confidence-thresholds",
		Description: "Create confidence_thresholds table for Phase 6 adaptive auto-approve thresholds",
		Apply: func() error {
			database, err := NewDatabase()
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer database.Close()

			_, err = database.db.Exec(`
				CREATE TABLE IF NOT EXISTS confidence_thresholds (
					id            INTEGER PRIMARY KEY AUTOINCREMENT,
					action_type   TEXT    NOT NULL,
					workspace     TEXT    NOT NULL DEFAULT '',
					threshold     REAL    NOT NULL DEFAULT 0.70,
					approvals     INTEGER NOT NULL DEFAULT 0,
					rejections    INTEGER NOT NULL DEFAULT 0,
					last_updated  DATETIME NOT NULL DEFAULT (datetime('now'))
				);
				CREATE UNIQUE INDEX IF NOT EXISTS idx_thresholds_type_ws
					ON confidence_thresholds(action_type, workspace);
			`)
			return err
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
	envPath := cfg.ResolveEnvFilePath()
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

// devtrackDataHome returns the XDG data home directory for DevTrack.
// Default: ~/.local/share/devtrack. Honours $XDG_DATA_HOME if set.
func devtrackDataHome() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "devtrack"), nil
}

// createDataDirectories creates all required Data/ subdirectories.
func createDataDirectories(dataDir string) error {
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "db"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "pids"),
		filepath.Join(dataDir, "configs"),
		filepath.Join(dataDir, "learning"),
		filepath.Join(dataDir, "learning", "chroma"),
		filepath.Join(dataDir, "reports"),
		filepath.Join(dataDir, "tls"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		fmt.Printf("  ✓ %s\n", d)
	}
	return nil
}

// createWorkspacesFile writes an initial workspaces.yaml with the workspace
// collected during setup. Skips if the file already exists.
func createWorkspacesFile(path, workspacePath, pmPlatform string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists — don't overwrite
	}
	if pmPlatform == "" || pmPlatform == "none" {
		pmPlatform = "none"
	}
	// Derive a short name from the last path component.
	name := filepath.Base(workspacePath)
	if name == "" || name == "." {
		name = "default"
	}
	content := "# workspaces.yaml — managed by DevTrack\n" +
		"# Add more workspaces with: devtrack workspace add <name> <path> [platform]\n" +
		"# pm_platform options: azure | github | gitlab | jira | none\n\n" +
		"version: \"1\"\nworkspaces:\n" +
		"  - name: \"" + name + "\"\n" +
		"    path: \"" + filepath.ToSlash(workspacePath) + "\"\n" +
		"    pm_platform: \"" + pmPlatform + "\"\n" +
		"    pm_project: \"\"\n" +
		"    enabled: true\n" +
		"    ignore_branches: []\n" +
		"    tags: []\n" +
		"    pm_assignee: \"\"\n" +
		"    pm_iteration_path: \"\"\n" +
		"    pm_area_path: \"\"\n" +
		"    pm_milestone: 0\n"
	return os.WriteFile(path, []byte(content), 0644)
}

// generateSecret returns a cryptographically random hex string of n bytes.
func generateSecret(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp-based (not cryptographic, but better than empty)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
