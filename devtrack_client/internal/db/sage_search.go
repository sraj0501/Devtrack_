package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
	"golang.org/x/text/cases"
)

// refreshSageEntries replaces a derived cache only after the whole Markdown
// snapshot is readable. Readers never receive stale results after a failed scan.
// SQLite serializes this with publication and merge; external editors must finish
// saving before requesting a consistent snapshot.
func (d *Database) refreshSageEntries(ctx context.Context, writer knowledge.Writer, read func(*sql.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := d.db.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE sage_publications SET attempts=attempts WHERE 0`); err != nil {
		return err
	}
	docs, err := writer.Snapshot()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sage_entries`); err != nil {
		return err
	}
	locations := make(map[string]string)
	fold := cases.Fold()
	for _, doc := range docs {
		topic := strings.TrimSuffix(doc.Filename, ".md")
		_, err = tx.ExecContext(ctx, `INSERT INTO sage_entries(filename,line,heading,body,commands,topic_fold,body_fold,heading_fold,commands_fold)
			VALUES(?,?,?,?,?,?,?,?,?)`, doc.Filename, doc.Line, doc.Heading, doc.Body, strings.Join(doc.Commands, "\n"), fold.String(topic), fold.String(topic+"\n"+doc.Body), fold.String(doc.Heading), fold.String(strings.Join(doc.Commands, " ")))
		if err != nil {
			return err
		}
		for _, sig := range doc.Signatures {
			if _, exists := locations[sig]; !exists {
				locations[sig] = doc.Filename
			}
		}
	}
	// Correct historical filenames after manual moves, merges, and deletions.
	// A missing file does not resurrect a deliberately deleted entry.
	rows, err := tx.QueryContext(ctx, `SELECT p.signature FROM sage_publications p JOIN sage_jobs j ON j.signature=p.signature WHERE p.state='documented' AND j.state='distilled'`)
	if err != nil {
		return err
	}
	var signatures []string
	for rows.Next() {
		var sig string
		if err := rows.Scan(&sig); err != nil {
			rows.Close()
			return err
		}
		signatures = append(signatures, sig)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, sig := range signatures {
		digest := sha256.Sum256([]byte(sig))
		if _, err := tx.ExecContext(ctx, `UPDATE sage_publications SET filename=? WHERE signature=?`, locations[hex.EncodeToString(digest[:])], sig); err != nil {
			return err
		}
	}
	if err := read(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

// SearchSageEntries uses literal AND substrings, including punctuation, and
// reference heading/command ranking. No model or raw-event fallback is involved.
func (d *Database) SearchSageEntries(ctx context.Context, writer knowledge.Writer, query, topic string, limit int) ([]knowledge.Entry, error) {
	fold := cases.Fold()
	terms := strings.Fields(fold.String(query))
	if len(query) > 4096 || len(topic) > 4096 || len(terms) > 64 {
		return nil, errors.New("Sage search query exceeds size limit")
	}
	if len(terms) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var entries []knowledge.Entry
	err := d.refreshSageEntries(ctx, writer, func(tx *sql.Tx) error {
		var score, filters []string
		var args, scoreArgs []any
		for _, term := range terms {
			score = append(score, `3*(instr(heading_fold,?)>0)+2*(instr(commands_fold,?)>0)`)
			scoreArgs = append(scoreArgs, term, term)
		}
		filters = append(filters, `instr(topic_fold,?)>0`)
		args = append(args, strings.TrimSuffix(fold.String(strings.TrimSpace(topic)), ".md"))
		for _, term := range terms {
			filters = append(filters, `instr(body_fold,?)>0`)
			args = append(args, term)
		}
		args = append(args, scoreArgs...)
		args = append(args, limit)
		rows, err := tx.QueryContext(ctx, `SELECT filename,line,heading,body,commands FROM sage_entries WHERE `+strings.Join(filters, " AND ")+` ORDER BY (`+strings.Join(score, "+")+`) DESC,filename,line LIMIT ?`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var entry knowledge.Entry
			if err := rows.Scan(&entry.Filename, &entry.Line, &entry.Heading, &entry.Body, &entry.Command); err != nil {
				return err
			}
			entry.Topic = strings.TrimSuffix(entry.Filename, ".md")
			entries = append(entries, entry)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (d *Database) ListSageEntryTopics(ctx context.Context, writer knowledge.Writer) ([]knowledge.Topic, error) {
	var topics []knowledge.Topic
	err := d.refreshSageEntries(ctx, writer, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT filename,count(*) FROM sage_entries GROUP BY filename ORDER BY filename`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var topic knowledge.Topic
			if err := rows.Scan(&topic.Name, &topic.Entries); err != nil {
				return err
			}
			topic.Name = strings.TrimSuffix(topic.Name, ".md")
			topics = append(topics, topic)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return topics, nil
}
