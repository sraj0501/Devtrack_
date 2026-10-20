package infra

import (
	"log"
	"path/filepath"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

func (im *IntegratedMonitor) resolveCommitMapping(ws *WorkspaceMonitor, evidence *ticket.ResolveInput) ticket.Result {
	r, err := ticket.NewResolver(ticket.ResolverConfig{TicketKey: ws.ticketKey, Kinds: ws.ticketKinds, BranchPattern: ws.ticketPattern})
	if err != nil {
		log.Printf("workspace %q ticket configuration invalid: %v", ws.workspaceName, err)
		return ticket.Result{Source: "none", State: "unlinked", Reason: "invalid workspace ticket configuration"}
	}
	if im.database != nil {
		active, err := im.database.GetActiveTicket(ws.workspaceName, ws.gitMonitor.repoPath)
		if err != nil {
			log.Printf("active ticket unavailable: %v", err)
		} else {
			evidence.ActiveTicket = active
			// Preserve explicit sessions created before durable overrides existed.
			if active == "" {
				session, err := im.database.GetActiveWorkSession()
				if err == nil && session != nil &&
					(session.RepoPath == "" || filepath.Clean(session.RepoPath) == filepath.Clean(ws.gitMonitor.repoPath)) &&
					(session.WorkspaceName == "" || session.WorkspaceName == ws.workspaceName) {
					evidence.ActiveTicket = session.TicketRef
				}
			}
		}
	}
	return r.Resolve(*evidence)
}
