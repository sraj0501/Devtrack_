package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestInitSchemaRollsBackFailure(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	// A legacy view allows the initial IF NOT EXISTS statement to succeed,
	// but cannot accept the later index creation.
	if _, err := sqlDB.Exec(`CREATE VIEW deferred_commits AS SELECT 1 AS status`); err != nil {
		t.Fatal(err)
	}
	d := &Database{db: sqlDB}
	if err := d.initSchema(); err == nil {
		t.Fatal("expected incompatible legacy schema to fail")
	}
	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed initialization left %d tables behind", count)
	}
	if _, err := sqlDB.Exec(`DROP VIEW deferred_commits`); err != nil {
		t.Fatal(err)
	}
	if err := d.initSchema(); err != nil {
		t.Fatalf("retry after repair: %v", err)
	}
}

func BenchmarkDatabaseInitialization(b *testing.B) {
	for _, reopen := range []bool{false, true} {
		name := "Fresh"
		if reopen {
			name = "Reopen"
		}
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				path := filepath.Join(b.TempDir(), "init.db")
				if reopen {
					d, err := NewDatabaseAtPath(path)
					if err != nil {
						b.Fatal(err)
					}
					d.db.Close()
				}
				b.StartTimer()
				d, err := NewDatabaseAtPath(path)
				if err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				d.db.Close()
			}
		})
	}
}
