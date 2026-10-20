package db

import (
	"context"
	"database/sql"
)

// schemaTransaction owns one connection for an immediate transaction. Taking
// the write lock before reading the schema allows busy_timeout to wait for
// other writers instead of failing a deferred read-to-write upgrade.
type schemaTransaction struct {
	conn *sql.Conn
	done bool
}

func (d *Database) beginSchemaTransaction() (*schemaTransaction, error) {
	conn, err := d.db.Conn(context.Background())
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		conn.Close()
		return nil, err
	}
	return &schemaTransaction{conn: conn}, nil
}

func (tx *schemaTransaction) Exec(query string, args ...any) (sql.Result, error) {
	return tx.conn.ExecContext(context.Background(), query, args...)
}

func (tx *schemaTransaction) Commit() error {
	if _, err := tx.Exec("COMMIT"); err != nil {
		return err // Deferred Rollback still owns the connection on failure.
	}
	tx.done = true
	return tx.conn.Close()
}

func (tx *schemaTransaction) Rollback() {
	if !tx.done {
		_, _ = tx.Exec("ROLLBACK")
		tx.done = true
		_ = tx.conn.Close()
	}
}
