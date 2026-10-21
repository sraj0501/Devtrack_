package infra

import (
	"log"

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
		}
	}
	return r.Resolve(*evidence)
}
