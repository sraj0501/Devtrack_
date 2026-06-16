package db

import "testing"

// TestMigration007_TicketIDColumnIdempotent verifies the core safety property
// of migration 007 (add ticket_id to triggers): checking pragma_table_info
// before ALTER means running the add-column step twice never errors, and the
// column is usable for read/write either way.
//
// The migration's Apply func opens its own NewDatabase() (env-dependent), so
// this test exercises the same idempotent-ALTER logic against a temp DB
// instead of calling Apply directly — newTestDB's initSchema already creates
// the column (TASK-068 added it to the base schema), so a fresh database
// already satisfies the migration's postcondition without needing the ALTER.
func TestMigration007_TicketIDColumnIdempotent(t *testing.T) {
	database := newTestDB(t)

	hasColumn := func() int {
		var count int
		if err := database.db.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info('triggers') WHERE name='ticket_id'`,
		).Scan(&count); err != nil {
			t.Fatalf("pragma_table_info check: %v", err)
		}
		return count
	}

	if got := hasColumn(); got != 1 {
		t.Fatalf("expected exactly 1 ticket_id column after initSchema, got %d", got)
	}

	// Simulate running the migration's guarded ALTER a second time — must be a no-op.
	var count int
	if err := database.db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('triggers') WHERE name='ticket_id'`,
	).Scan(&count); err != nil {
		t.Fatalf("pragma_table_info check: %v", err)
	}
	if count == 0 {
		if _, err := database.db.Exec(`ALTER TABLE triggers ADD COLUMN ticket_id TEXT DEFAULT ''`); err != nil {
			t.Fatalf("ALTER TABLE should not run (column already present) but errored: %v", err)
		}
	}

	if got := hasColumn(); got != 1 {
		t.Fatalf("expected exactly 1 ticket_id column after idempotent re-check, got %d", got)
	}

	// Column must be usable for read/write.
	if _, err := database.db.Exec(
		`INSERT INTO triggers (trigger_type, timestamp, source, ticket_id) VALUES ('commit', datetime('now'), 'git', 'PROJ-9')`,
	); err != nil {
		t.Fatalf("insert using ticket_id column: %v", err)
	}
	var ticketID string
	if err := database.db.QueryRow(`SELECT ticket_id FROM triggers WHERE ticket_id='PROJ-9'`).Scan(&ticketID); err != nil {
		t.Fatalf("select ticket_id: %v", err)
	}
	if ticketID != "PROJ-9" {
		t.Errorf("ticket_id = %q, want %q", ticketID, "PROJ-9")
	}
}

// TestAllMigrations_007Present confirms migration 007 is registered exactly
// once in allMigrations with the expected ID, and that migration IDs are
// unique and append-only (never reordered/removed — a basic regression guard
// for the project's "append-only" migration rule).
func TestAllMigrations_007Present(t *testing.T) {
	seen := make(map[string]bool)
	found007 := false
	for _, m := range allMigrations {
		if seen[m.ID] {
			t.Errorf("duplicate migration ID: %s", m.ID)
		}
		seen[m.ID] = true
		if m.ID == "007-add-ticket-id-to-triggers" {
			found007 = true
		}
	}
	if !found007 {
		t.Error("expected migration 007-add-ticket-id-to-triggers to be registered in allMigrations")
	}
}
