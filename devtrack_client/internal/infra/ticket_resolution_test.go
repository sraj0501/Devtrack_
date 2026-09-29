package infra

import (
	"path/filepath"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

func TestDurableActiveTicketRespectsBranchPrecedenceAndScope(t *testing.T) {
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "tickets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	cfg := ticket.ResolverConfig{TicketKey: "TASK"}
	if err := database.SetActiveTicket("test", "/repo", "TASK-159", cfg); err != nil {
		t.Fatal(err)
	}
	monitor := IntegratedMonitor{database: database}
	ws := &WorkspaceMonitor{workspaceName: "test", ticketKey: "TASK", gitMonitor: &GitMonitor{repoPath: "/repo"}}
	got := monitor.resolveCommitMapping(ws, &ticket.ResolveInput{Branch: "main"})
	if got.TicketID != "TASK-159" || got.Source != ticket.SourceActiveTicket {
		t.Fatalf("override: %+v", got)
	}
	got = monitor.resolveCommitMapping(ws, &ticket.ResolveInput{Branch: "fix/TASK-160-bug"})
	if got.TicketID != "TASK-160" || !got.Conflict {
		t.Fatalf("branch precedence: %+v", got)
	}
	ws.workspaceName = "other"
	if got := monitor.resolveCommitMapping(ws, &ticket.ResolveInput{Branch: "main"}); got.TicketID != "" {
		t.Fatalf("scope leaked: %+v", got)
	}
	if err := database.SetActiveTicket("test", "/repo", "OTHER-1", cfg); err == nil {
		t.Fatal("accepted wrong namespace")
	}
	if err := database.ClearActiveTicket("test", "/repo"); err != nil {
		t.Fatal(err)
	}
	ws.workspaceName = "test"
	if got := monitor.resolveCommitMapping(ws, &ticket.ResolveInput{Branch: "main"}); got.TicketID != "" {
		t.Fatalf("clear: %+v", got)
	}
}
