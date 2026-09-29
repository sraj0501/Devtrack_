package infra

import (
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

func TestWorkspaceMonitorCanonicalTicketResolution(t *testing.T) {
	ws := &WorkspaceMonitor{ticketKey: "PROJ"}
	resolver, err := ticket.NewResolver(ticket.ResolverConfig{
		TicketKey: ws.ticketKey, Kinds: ws.ticketKinds, BranchPattern: ws.ticketPattern,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.Resolve(ticket.ResolveInput{Branch: "feature/PROJ-123-add-login"})
	if got.TicketID != "PROJ-123" || got.Source != ticket.SourceBranch {
		t.Fatalf("Resolve() = %#v", got)
	}
}

func TestWorkspaceMonitorNonCanonicalBranchIsUnlinked(t *testing.T) {
	resolver, err := ticket.NewResolver(ticket.ResolverConfig{TicketKey: "PROJ"})
	if err != nil {
		t.Fatal(err)
	}
	for _, branch := range []string{"main", "chore/update-readme", "prefix/feature/PROJ-1-slug"} {
		got := resolver.Resolve(ticket.ResolveInput{Branch: branch})
		if got.TicketID != "" || got.State != ticket.StateUnlinked {
			t.Errorf("Resolve(%q) = %#v", branch, got)
		}
	}
}

func TestWorkspaceMonitorCustomPattern(t *testing.T) {
	resolver, err := ticket.NewResolver(ticket.ResolverConfig{
		TicketKey: "DT", BranchPattern: `^work/(?P<ticket>DT-[1-9][0-9]*)/[a-z]+$`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resolver.Resolve(ticket.ResolveInput{Branch: "work/DT-999/thing"}); got.TicketID != "DT-999" {
		t.Fatalf("Resolve() = %#v", got)
	}
}

func TestTriggerEventCarriesMappingProvenance(t *testing.T) {
	event := TriggerEvent{
		TicketID: "PROJ-123", TicketSource: "branch", TicketState: "linked", TicketConfidence: 1,
	}
	if event.TicketID != "PROJ-123" || event.TicketSource != "branch" || event.TicketState != "linked" {
		t.Fatalf("event = %#v", event)
	}
}

func TestWorkspaceKeyIncludesTicketContract(t *testing.T) {
	base := config.WorkspaceConfig{Name: "repo", Path: "/repo", TicketKey: "PROJ"}
	changed := base
	changed.TicketKey = "OTHER"
	if workspaceKey(base) == workspaceKey(changed) {
		t.Fatal("ticket_key change must restart the workspace monitor")
	}
	changed = base
	changed.TicketPattern = `^work/(?P<ticket>PROJ-[1-9][0-9]*)/[a-z]+$`
	if workspaceKey(base) == workspaceKey(changed) {
		t.Fatal("ticket_pattern change must restart the workspace monitor")
	}
}

func TestConflictedMappingCannotSendOutbound(t *testing.T) {
	data := &trigger.CommitTriggerData{TicketID: "PROJ-1", TicketConflict: true}
	event := TriggerEvent{TicketID: "PROJ-1", TicketConflict: true, SuppressOutbound: true}
	if shouldSendCommitOutbound(data, event) {
		t.Fatal("conflicted mapping must not execute an outbound trigger")
	}
	event.SuppressOutbound = false
	if shouldSendCommitOutbound(data, event) {
		t.Fatal("ticket conflict must independently prevent outbound execution")
	}
}
