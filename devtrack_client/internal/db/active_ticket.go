package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

func (d *Database) SetActiveTicket(workspace, repo, reference string, contract ticket.ResolverConfig) error {
	if workspace == "" || repo == "" {
		return errors.New("active ticket requires workspace and repository")
	}
	r, err := ticket.NewResolver(contract)
	if err != nil {
		return err
	}
	if r.Resolve(ticket.ResolveInput{ActiveTicket: reference}).TicketID != reference || reference == "" {
		return errors.New("active ticket does not match workspace namespace")
	}
	_, err = d.db.Exec(`INSERT INTO active_ticket_overrides(workspace,repo_path,ticket_id,selected_at) VALUES(?,?,?,?) ON CONFLICT(workspace,repo_path) DO UPDATE SET ticket_id=excluded.ticket_id,selected_at=excluded.selected_at`, workspace, filepath.Clean(repo), reference, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (d *Database) GetActiveTicket(workspace, repo string) (string, error) {
	var ref string
	err := d.db.QueryRow(`SELECT ticket_id FROM active_ticket_overrides WHERE workspace=? AND repo_path=?`, workspace, filepath.Clean(repo)).Scan(&ref)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return ref, err
}

func (d *Database) ClearActiveTicket(workspace, repo string) error {
	_, err := d.db.Exec(`DELETE FROM active_ticket_overrides WHERE workspace=? AND repo_path=?`, workspace, filepath.Clean(repo))
	return err
}
