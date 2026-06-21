package db

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Test: UpsertSkill — insert then upsert, verify evidence_count updated,
//       promoted_at unchanged on second upsert.
// ---------------------------------------------------------------------------

func TestSkillUpsertAndList(t *testing.T) {
	// Reuse the existing test helper which creates a temp SQLite DB with the
	// inferences, corrections, and confidence_thresholds tables.  We create the
	// skills table manually here to mirror the migration 011 DDL.
	database := newInferencesTestDB(t)

	// Create the skills table (mirrors migration 011).
	_, err := database.db.Exec(`
		CREATE TABLE IF NOT EXISTS skills (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT    NOT NULL UNIQUE,
			description    TEXT    NOT NULL,
			context_type   TEXT    NOT NULL,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			promoted_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			last_seen_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		t.Fatalf("create skills table: %v", err)
	}

	// UpsertSkill — first insert.
	err = database.UpsertSkill("imperative_tone", "Dev uses imperative mood", "commit", 5)
	if err != nil {
		t.Fatalf("UpsertSkill (insert): %v", err)
	}

	// Fetch first version to capture promoted_at.
	skill1, err := database.GetSkill("imperative_tone")
	if err != nil {
		t.Fatalf("GetSkill after insert: %v", err)
	}
	if skill1 == nil {
		t.Fatal("GetSkill: expected skill, got nil")
	}
	if skill1.EvidenceCount != 5 {
		t.Errorf("EvidenceCount after insert: got %d, want 5", skill1.EvidenceCount)
	}
	firstPromotedAt := skill1.PromotedAt

	// UpsertSkill — same name with higher evidence_count.
	err = database.UpsertSkill("imperative_tone", "Dev uses imperative mood", "commit", 8)
	if err != nil {
		t.Fatalf("UpsertSkill (upsert): %v", err)
	}

	// ListSkills — should return 1 skill with evidence_count=8.
	skills, err := database.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].EvidenceCount != 8 {
		t.Errorf("EvidenceCount after upsert: got %d, want 8", skills[0].EvidenceCount)
	}

	// promoted_at should be stable — not updated by the second upsert.
	// SQLite datetime precision is second-level, so they should match when upsert
	// runs within the same second.  The ON CONFLICT clause does NOT touch promoted_at.
	if !skills[0].PromotedAt.Equal(firstPromotedAt) {
		// Allow small clock drift (tests run fast, but guard against sub-second
		// precision differences in SQLite vs Go time parsing).
		diff := skills[0].PromotedAt.Sub(firstPromotedAt)
		if diff < 0 {
			diff = -diff
		}
		if diff.Seconds() > 1 {
			t.Errorf("promoted_at changed unexpectedly: first=%v, after upsert=%v",
				firstPromotedAt, skills[0].PromotedAt)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: GetSkill — returns nil, nil for unknown name.
// ---------------------------------------------------------------------------

func TestGetSkillNotFound(t *testing.T) {
	database := newInferencesTestDB(t)

	_, err := database.db.Exec(`
		CREATE TABLE IF NOT EXISTS skills (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT    NOT NULL UNIQUE,
			description    TEXT    NOT NULL,
			context_type   TEXT    NOT NULL,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			promoted_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			last_seen_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		t.Fatalf("create skills table: %v", err)
	}

	skill, err := database.GetSkill("nonexistent_skill")
	if err != nil {
		t.Fatalf("GetSkill: unexpected error: %v", err)
	}
	if skill != nil {
		t.Errorf("GetSkill: expected nil for unknown name, got %+v", skill)
	}
}

// ---------------------------------------------------------------------------
// Test: UpsertSkill — lower evidence_count on second call is ignored (MAX).
// ---------------------------------------------------------------------------

func TestSkillUpsertKeepsHigherEvidenceCount(t *testing.T) {
	database := newInferencesTestDB(t)

	_, err := database.db.Exec(`
		CREATE TABLE IF NOT EXISTS skills (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT    NOT NULL UNIQUE,
			description    TEXT    NOT NULL,
			context_type   TEXT    NOT NULL,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			promoted_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			last_seen_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		t.Fatalf("create skills table: %v", err)
	}

	// Insert with high evidence_count.
	if err := database.UpsertSkill("bracket_prefix", "Ticket bracket prefix pattern", "commit", 10); err != nil {
		t.Fatalf("UpsertSkill (first): %v", err)
	}
	// Upsert with lower evidence_count — MAX() should keep 10.
	if err := database.UpsertSkill("bracket_prefix", "Ticket bracket prefix pattern", "commit", 3); err != nil {
		t.Fatalf("UpsertSkill (second): %v", err)
	}

	skill, err := database.GetSkill("bracket_prefix")
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if skill == nil {
		t.Fatal("GetSkill: expected skill, got nil")
	}
	if skill.EvidenceCount != 10 {
		t.Errorf("EvidenceCount: got %d, want 10 (MAX preserved)", skill.EvidenceCount)
	}
}
