package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewDatabaseAtPathWaitsForConcurrentWriter(t *testing.T) {
	t.Setenv("SQLITE_BUSY_TIMEOUT_MS", "1000")
	dbPath := filepath.Join(t.TempDir(), "concurrent.db")

	first, err := NewDatabaseAtPath(dbPath)
	if err != nil {
		t.Fatalf("open first database: %v", err)
	}
	defer first.Close()

	tx, err := first.db.Begin()
	if err != nil {
		t.Fatalf("begin writer: %v", err)
	}
	if _, err := tx.Exec("INSERT INTO logs (timestamp, level, component, message) VALUES (?, ?, ?, ?)",
		time.Now(), "info", "test", "hold write lock"); err != nil {
		t.Fatalf("acquire write lock: %v", err)
	}

	released := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = tx.Commit()
		close(released)
	}()

	second, err := NewDatabaseAtPath(dbPath)
	if err != nil {
		t.Fatalf("second database should wait for writer: %v", err)
	}
	defer second.Close()
	<-released
}
