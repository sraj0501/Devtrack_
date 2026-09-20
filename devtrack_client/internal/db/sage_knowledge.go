package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	sageknowledge "github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

func (d *Database) createSageKnowledgeTables() error {
	var ftsAlreadyExists int
	if err := d.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'sage_knowledge_fts'
	`).Scan(&ftsAlreadyExists); err != nil {
		return fmt.Errorf("check sage knowledge index: %w", err)
	}
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS sage_knowledge (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			signature TEXT NOT NULL UNIQUE,
			topic TEXT NOT NULL,
			representative_command TEXT NOT NULL,
			project_id TEXT NOT NULL DEFAULT '',
			use_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			failure_count INTEGER NOT NULL DEFAULT 0,
			first_seen DATETIME NOT NULL,
			last_seen DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sage_knowledge_topic ON sage_knowledge(topic);
		CREATE INDEX IF NOT EXISTS idx_sage_knowledge_last_seen ON sage_knowledge(last_seen);

		CREATE TABLE IF NOT EXISTS sage_knowledge_sources (
			knowledge_id INTEGER NOT NULL,
			delivery_key TEXT NOT NULL UNIQUE,
			PRIMARY KEY (knowledge_id, delivery_key),
			FOREIGN KEY (knowledge_id) REFERENCES sage_knowledge(id),
			FOREIGN KEY (delivery_key) REFERENCES sage_events(delivery_key)
		);
		CREATE INDEX IF NOT EXISTS idx_sage_knowledge_sources_entry
			ON sage_knowledge_sources(knowledge_id);

		CREATE VIRTUAL TABLE IF NOT EXISTS sage_knowledge_fts USING fts5(
			signature,
			topic,
			representative_command,
			content='sage_knowledge',
			content_rowid='id'
		);
		CREATE TRIGGER IF NOT EXISTS sage_knowledge_ai AFTER INSERT ON sage_knowledge BEGIN
			INSERT INTO sage_knowledge_fts(rowid, signature, topic, representative_command)
			VALUES (new.id, new.signature, new.topic, new.representative_command);
		END;
		CREATE TRIGGER IF NOT EXISTS sage_knowledge_au AFTER UPDATE ON sage_knowledge BEGIN
			INSERT INTO sage_knowledge_fts(sage_knowledge_fts, rowid, signature, topic, representative_command)
			VALUES ('delete', old.id, old.signature, old.topic, old.representative_command);
			INSERT INTO sage_knowledge_fts(rowid, signature, topic, representative_command)
			VALUES (new.id, new.signature, new.topic, new.representative_command);
		END;
		CREATE TRIGGER IF NOT EXISTS sage_knowledge_ad AFTER DELETE ON sage_knowledge BEGIN
			INSERT INTO sage_knowledge_fts(sage_knowledge_fts, rowid, signature, topic, representative_command)
			VALUES ('delete', old.id, old.signature, old.topic, old.representative_command);
		END;
	`)
	if err != nil {
		return err
	}
	if err := d.backfillSageKnowledge(); err != nil {
		return err
	}
	if ftsAlreadyExists == 0 {
		if _, err := d.db.Exec(`INSERT INTO sage_knowledge_fts(sage_knowledge_fts) VALUES ('rebuild')`); err != nil {
			return fmt.Errorf("build sage knowledge index: %w", err)
		}
	}
	return nil
}

func upsertSageKnowledgeTx(tx *sql.Tx, event sage.Event, deliveryKey string) error {
	succeeded, failed := 0, 0
	if event.Success != nil {
		if *event.Success {
			succeeded = 1
		} else {
			failed = 1
		}
	}
	topic := sageknowledge.TopicForSignature(event.Signature)
	_, err := tx.Exec(`
		INSERT INTO sage_knowledge (
			signature, topic, representative_command, project_id,
			use_count, success_count, failure_count, first_seen, last_seen
		) VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(signature) DO UPDATE SET
			topic = excluded.topic,
			representative_command = CASE
				WHEN excluded.representative_command < sage_knowledge.representative_command
				THEN excluded.representative_command ELSE sage_knowledge.representative_command END,
			project_id = CASE
				WHEN sage_knowledge.project_id = '' THEN excluded.project_id
				WHEN excluded.project_id = '' THEN sage_knowledge.project_id
				WHEN excluded.project_id < sage_knowledge.project_id THEN excluded.project_id
				ELSE sage_knowledge.project_id END,
			use_count = sage_knowledge.use_count + 1,
			success_count = sage_knowledge.success_count + excluded.success_count,
			failure_count = sage_knowledge.failure_count + excluded.failure_count,
			first_seen = MIN(sage_knowledge.first_seen, excluded.first_seen),
			last_seen = MAX(sage_knowledge.last_seen, excluded.last_seen)
	`, event.Signature, topic, event.Command, event.ProjectID, succeeded, failed, event.OccurredAt.UTC(), event.OccurredAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert sage knowledge: %w", err)
	}
	var knowledgeID int64
	if err := tx.QueryRow(`SELECT id FROM sage_knowledge WHERE signature = ?`, event.Signature).Scan(&knowledgeID); err != nil {
		return fmt.Errorf("find sage knowledge: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO sage_knowledge_sources (knowledge_id, delivery_key)
		VALUES (?, ?)
	`, knowledgeID, deliveryKey); err != nil {
		return fmt.Errorf("link sage knowledge source: %w", err)
	}
	return nil
}

func (d *Database) backfillSageKnowledge() error {
	rows, err := d.db.Query(`
		SELECT e.delivery_key, e.schema_version, e.event_id, e.harness, e.session_id,
		       e.event_type, e.tool, CAST(e.occurred_at AS TEXT), e.project_id,
		       e.command, e.signature, e.success, e.exit_code
		FROM sage_events e
		LEFT JOIN sage_knowledge_sources s ON s.delivery_key = e.delivery_key
		WHERE s.delivery_key IS NULL
		ORDER BY e.delivery_key
	`)
	if err != nil {
		return fmt.Errorf("find unindexed sage events: %w", err)
	}
	type pendingEvent struct {
		deliveryKey string
		event       sage.Event
	}
	var pending []pendingEvent
	for rows.Next() {
		var item pendingEvent
		var occurredAt string
		var success sql.NullBool
		var exitCode sql.NullInt64
		if err := rows.Scan(&item.deliveryKey, &item.event.SchemaVersion, &item.event.EventID,
			&item.event.Harness, &item.event.SessionID, &item.event.EventType, &item.event.Tool,
			&occurredAt, &item.event.ProjectID, &item.event.Command, &item.event.Signature,
			&success, &exitCode); err != nil {
			rows.Close()
			return fmt.Errorf("scan unindexed sage event: %w", err)
		}
		item.event.OccurredAt = parseTimestamp(occurredAt)
		if item.event.OccurredAt.IsZero() {
			rows.Close()
			return errors.New("unindexed sage event has invalid timestamp")
		}
		if success.Valid {
			value := success.Bool
			item.event.Success = &value
		}
		if exitCode.Valid {
			value := int(exitCode.Int64)
			item.event.ExitCode = &value
		}
		pending = append(pending, item)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close unindexed sage events: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read unindexed sage events: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin sage knowledge backfill: %w", err)
	}
	defer tx.Rollback()
	for _, item := range pending {
		if err := upsertSageKnowledgeTx(tx, item.event, item.deliveryKey); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sage knowledge backfill: %w", err)
	}
	return nil
}

// SearchSageKnowledge returns deterministic, privacy-minimized command records.
func (d *Database) SearchSageKnowledge(query, topic string, limit int) ([]sageknowledge.Entry, error) {
	expression := sageknowledge.SearchExpression(query)
	if expression == "" {
		return nil, errors.New("sage search query has no searchable terms")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if topic != "" {
		parts := strings.Fields(topic)
		if len(parts) != 1 || sageknowledge.SearchExpression(topic) == "" {
			return nil, errors.New("sage topic is invalid")
		}
		topic = sageknowledge.TopicForSignature(topic)
	}
	rows, err := d.db.Query(`
		SELECT k.id, k.signature, k.topic, k.representative_command, k.project_id,
		       k.use_count, k.success_count, k.failure_count,
		       CAST(k.first_seen AS TEXT), CAST(k.last_seen AS TEXT)
		FROM sage_knowledge_fts
		JOIN sage_knowledge k ON k.id = sage_knowledge_fts.rowid
		WHERE sage_knowledge_fts MATCH ? AND (? = '' OR k.topic = ?)
		ORDER BY bm25(sage_knowledge_fts), k.signature
		LIMIT ?
	`, expression, topic, topic, limit)
	if err != nil {
		return nil, fmt.Errorf("search sage knowledge: %w", err)
	}
	defer rows.Close()
	var entries []sageknowledge.Entry
	for rows.Next() {
		var entry sageknowledge.Entry
		var firstSeen, lastSeen string
		if err := rows.Scan(&entry.ID, &entry.Signature, &entry.Topic, &entry.Command, &entry.ProjectID,
			&entry.UseCount, &entry.SuccessCount, &entry.FailureCount, &firstSeen, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan sage knowledge: %w", err)
		}
		entry.FirstSeen = parseTimestamp(firstSeen)
		entry.LastSeen = parseTimestamp(lastSeen)
		entry.SourceHarnesses, err = d.sageKnowledgeHarnesses(entry.ID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (d *Database) sageKnowledgeHarnesses(knowledgeID int64) ([]string, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT e.harness
		FROM sage_knowledge_sources s
		JOIN sage_events e ON e.delivery_key = s.delivery_key
		WHERE s.knowledge_id = ?
		ORDER BY e.harness
	`, knowledgeID)
	if err != nil {
		return nil, fmt.Errorf("list sage knowledge sources: %w", err)
	}
	defer rows.Close()
	var harnesses []string
	for rows.Next() {
		var harness string
		if err := rows.Scan(&harness); err != nil {
			return nil, fmt.Errorf("scan sage knowledge source: %w", err)
		}
		harnesses = append(harnesses, harness)
	}
	return harnesses, rows.Err()
}

// ListSageTopics returns stable aggregate counts for every command family.
func (d *Database) ListSageTopics() ([]sageknowledge.Topic, error) {
	rows, err := d.db.Query(`
		SELECT topic, COUNT(*), SUM(use_count), CAST(MAX(last_seen) AS TEXT)
		FROM sage_knowledge
		GROUP BY topic
		ORDER BY topic
	`)
	if err != nil {
		return nil, fmt.Errorf("list sage topics: %w", err)
	}
	defer rows.Close()
	var topics []sageknowledge.Topic
	for rows.Next() {
		var topic sageknowledge.Topic
		var lastSeen string
		if err := rows.Scan(&topic.Name, &topic.Entries, &topic.Uses, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan sage topic: %w", err)
		}
		topic.LastSeen = parseTimestamp(lastSeen)
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}
