package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

type TicketMapping struct {
	TriggerID int64             `json:"trigger_id"`
	Workspace string            `json:"workspace"`
	Version   int64             `json:"version"`
	Original  ticket.Resolution `json:"original"`
	Effective ticket.Resolution `json:"effective"`
	Evidence  ticket.Evidence   `json:"evidence"`
	Contract  ticket.Contract   `json:"contract"`
}

type TicketCorrection struct {
	ID          int64             `json:"id"`
	TriggerID   int64             `json:"trigger_id"`
	RequestID   string            `json:"request_id"`
	Previous    ticket.Resolution `json:"previous"`
	Replacement ticket.Resolution `json:"replacement"`
	Actor       string            `json:"actor"`
	Channel     string            `json:"channel"`
	Reason      string            `json:"reason"`
	Timestamp   string            `json:"timestamp"`
}

type CorrectTicketRequest struct {
	TriggerID                         int64
	Reference                         string
	Contract                          ticket.Contract
	Actor, Channel, Reason, RequestID string
	ExpectedVersion                   int64 // -1 permits serialized correction of the current version.
}

var ErrMappingChanged = errors.New("ticket mapping changed; inspect it before correcting again")

func (d *Database) initTicketMappings() error {
	_, err := d.db.Exec(`
	CREATE TABLE IF NOT EXISTS ticket_mappings (
	 trigger_id INTEGER PRIMARY KEY REFERENCES triggers(id), workspace TEXT NOT NULL DEFAULT '',
	 original_json TEXT NOT NULL, effective_json TEXT NOT NULL,
	 evidence_json TEXT NOT NULL DEFAULT '{}', contract_json TEXT NOT NULL DEFAULT '{}',
	 version INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS ticket_mapping_corrections (
	 id INTEGER PRIMARY KEY AUTOINCREMENT, trigger_id INTEGER NOT NULL REFERENCES triggers(id),
	 request_id TEXT NOT NULL UNIQUE, previous_json TEXT NOT NULL, replacement_json TEXT NOT NULL,
	 actor TEXT NOT NULL, channel TEXT NOT NULL, reason TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_ticket_corrections_trigger ON ticket_mapping_corrections(trigger_id,id);
	CREATE TRIGGER IF NOT EXISTS ticket_original_immutable BEFORE UPDATE OF original_json,evidence_json,contract_json,workspace ON ticket_mappings
	BEGIN SELECT RAISE(ABORT, 'original ticket evidence is immutable'); END;
	CREATE TRIGGER IF NOT EXISTS ticket_correction_no_update BEFORE UPDATE ON ticket_mapping_corrections
	BEGIN SELECT RAISE(ABORT, 'ticket correction history is append-only'); END;
	CREATE TRIGGER IF NOT EXISTS ticket_correction_no_delete BEFORE DELETE ON ticket_mapping_corrections
	BEGIN SELECT RAISE(ABORT, 'ticket correction history is append-only'); END;
	INSERT OR IGNORE INTO ticket_mappings(trigger_id,original_json,effective_json)
	SELECT id,
	 json_object('ticket_id',COALESCE(ticket_id,''),'source','legacy','confidence',0,'state',CASE WHEN COALESCE(ticket_id,'')='' THEN 'unlinked' ELSE 'legacy' END),
	 json_object('ticket_id',COALESCE(ticket_id,''),'source','legacy','confidence',0,'state',CASE WHEN COALESCE(ticket_id,'')='' THEN 'unlinked' ELSE 'legacy' END)
	FROM triggers WHERE trigger_type='commit';
	`)
	return err
}

func insertTicketMapping(tx *sql.Tx, id int64, record TriggerRecord) error {
	resolution := ticket.Resolution{TicketID: record.TicketID, Source: "legacy", State: "legacy"}
	if record.TicketID == "" {
		resolution.State = "unlinked"
	}
	if record.TicketMapping != nil {
		resolution = *record.TicketMapping
	}
	encoded, err := json.Marshal(resolution)
	if err != nil {
		return err
	}
	evidence, err := json.Marshal(record.TicketEvidence)
	if err != nil {
		return err
	}
	contract, err := json.Marshal(record.TicketContract)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO ticket_mappings(trigger_id,workspace,original_json,effective_json,evidence_json,contract_json) VALUES(?,?,?,?,?,?)`, id, record.WorkspaceName, string(encoded), string(encoded), string(evidence), string(contract))
	return err
}

type mappingScanner interface{ Scan(...any) error }

func scanTicketMapping(row mappingScanner) (*TicketMapping, error) {
	var m TicketMapping
	var original, effective, evidence, contract string
	if err := row.Scan(&m.TriggerID, &m.Workspace, &m.Version, &original, &effective, &evidence, &contract); err != nil {
		return nil, err
	}
	for _, pair := range []struct {
		raw    string
		target any
	}{{original, &m.Original}, {effective, &m.Effective}, {evidence, &m.Evidence}, {contract, &m.Contract}} {
		if err := json.Unmarshal([]byte(pair.raw), pair.target); err != nil {
			return nil, fmt.Errorf("invalid stored ticket mapping: %w", err)
		}
	}
	return &m, nil
}

const mappingSelect = `SELECT trigger_id,workspace,version,original_json,effective_json,evidence_json,contract_json FROM ticket_mappings WHERE trigger_id=?`

func (d *Database) GetTicketMapping(triggerID int64) (*TicketMapping, error) {
	return scanTicketMapping(d.db.QueryRow(mappingSelect, triggerID))
}

// FindCommitTrigger requires repository identity and an unambiguous hash prefix.
// '%' and '_' are literals here, not LIKE wildcards.
func (d *Database) FindCommitTrigger(repoPath, hash string) (int64, error) {
	if repoPath == "" || len(hash) < 7 {
		return 0, errors.New("repository and at least seven commit hash characters are required")
	}
	rows, err := d.db.Query(`SELECT id FROM triggers WHERE trigger_type='commit' AND repo_path=? AND substr(commit_hash,1,?)=? ORDER BY id LIMIT 2`, repoPath, len(hash), hash)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, errors.New("commit is missing or ambiguous; use the full hash in its repository")
	}
	return ids[0], nil
}

// CorrectTicketMapping locks before reading, preserving a serial audit chain
// even when CLI and another correction channel submit at the same time.
func (d *Database) CorrectTicketMapping(req CorrectTicketRequest) (*TicketMapping, error) {
	if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.Actor) == "" || strings.TrimSpace(req.Channel) == "" {
		return nil, errors.New("correction requires request ID, actor and channel")
	}
	r, err := ticket.NewResolver(req.Contract)
	if err != nil {
		return nil, err
	}
	if !r.ValidReference(req.Reference) {
		return nil, errors.New("replacement does not match the workspace ticket namespace")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE ticket_mappings SET version=version WHERE trigger_id=?`, req.TriggerID); err != nil {
		return nil, err
	}
	m, err := scanTicketMapping(tx.QueryRow(mappingSelect, req.TriggerID))
	if err != nil {
		return nil, err
	}
	var priorID int64
	var priorReplacement, priorActor, priorChannel string
	err = tx.QueryRow(`SELECT trigger_id,replacement_json,actor,channel FROM ticket_mapping_corrections WHERE request_id=?`, req.RequestID).Scan(&priorID, &priorReplacement, &priorActor, &priorChannel)
	if err == nil {
		var prior ticket.Resolution
		if err := json.Unmarshal([]byte(priorReplacement), &prior); err != nil {
			return nil, err
		}
		if priorID != req.TriggerID || prior.TicketID != req.Reference || priorActor != req.Actor || priorChannel != req.Channel {
			return nil, errors.New("correction request ID was already used for a different request")
		}
		return m, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if req.ExpectedVersion >= 0 && req.ExpectedVersion != m.Version {
		return nil, ErrMappingChanged
	}
	previous, err := json.Marshal(m.Effective)
	if err != nil {
		return nil, err
	}
	next := ticket.Resolution{TicketID: req.Reference, ExternalID: r.ExternalID(req.Reference), Source: "correction", Confidence: 1, State: "corrected", Branch: m.Original.Branch}
	replacement, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO ticket_mapping_corrections(trigger_id,request_id,previous_json,replacement_json,actor,channel,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`, req.TriggerID, req.RequestID, string(previous), string(replacement), req.Actor, req.Channel, req.Reason, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE ticket_mappings SET effective_json=?,version=version+1 WHERE trigger_id=?`, string(replacement), req.TriggerID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE triggers SET ticket_id=? WHERE id=?`, req.Reference, req.TriggerID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	m.Effective = next
	m.Version++
	return m, nil
}

func (d *Database) ListTicketCorrections(triggerID int64) ([]TicketCorrection, error) {
	rows, err := d.db.Query(`SELECT id,trigger_id,request_id,previous_json,replacement_json,actor,channel,reason,created_at FROM ticket_mapping_corrections WHERE trigger_id=? ORDER BY id`, triggerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []TicketCorrection{}
	for rows.Next() {
		var c TicketCorrection
		var previous, replacement string
		if err := rows.Scan(&c.ID, &c.TriggerID, &c.RequestID, &previous, &replacement, &c.Actor, &c.Channel, &c.Reason, &c.Timestamp); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(previous), &c.Previous); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(replacement), &c.Replacement); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

type TicketMappingCounts struct{ Unlinked, Conflicts, Legacy int }

func (d *Database) CountTicketMappings(repoPath string) (TicketMappingCounts, error) {
	var counts TicketMappingCounts
	err := d.db.QueryRow(`SELECT COALESCE(SUM(json_extract(m.effective_json,'$.state')='unlinked'),0),COALESCE(SUM(json_extract(m.effective_json,'$.conflict')=1),0),COALESCE(SUM(json_extract(m.effective_json,'$.state')='legacy'),0) FROM ticket_mappings m JOIN triggers t ON t.id=m.trigger_id WHERE (?='' OR t.repo_path=?)`, repoPath, repoPath).Scan(&counts.Unlinked, &counts.Conflicts, &counts.Legacy)
	return counts, err
}
