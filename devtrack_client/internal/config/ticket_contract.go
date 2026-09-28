package config

import (
	"fmt"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

// TicketContract provides visible, deterministic defaults for older workspace
// files. Save persists the effective key. Explicit invalid settings never widen
// matching; Load returns an error so daemon reload retains its last valid state.
func (ws WorkspaceConfig) TicketContract() ticket.Contract {
	key := ws.TicketKey
	if key == "" {
		switch ws.PMPlatform {
		case "github":
			key = "GH"
		case "gitlab":
			key = "GL"
		case "azure":
			key = "ADO"
		case "jira":
			key = ws.PMProject
			if key == "" {
				key = "PROJ"
			}
		default:
			key = "PROJ"
		}
	}
	return ticket.Contract{Key: key, Provider: ws.PMPlatform, Pattern: ws.TicketPattern}
}

func (wc *WorkspacesConfig) ValidateTickets() error {
	for _, ws := range wc.Workspaces {
		if _, err := ticket.NewResolver(ws.TicketContract()); err != nil {
			return fmt.Errorf("workspace %q ticket configuration: %w", ws.Name, err)
		}
	}
	return nil
}

func (ws WorkspaceConfig) TicketConvention() string {
	c := ws.TicketContract()
	if c.Pattern != "" {
		return c.Pattern
	}
	return "<kind>/" + c.Key + "-<positive-number>-<lowercase-kebab-slug>"
}
