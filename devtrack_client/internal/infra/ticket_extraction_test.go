package infra

import (
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

// TestWorkspaceMonitor_TicketPattern_DefaultExtraction mirrors the extraction
// call in handleCommitForWorkspace (TASK-068): a WorkspaceMonitor with no
// custom ticketPattern falls back to the default patterns, so a branch named
// after a Jira/ADO-style ticket produces a populated TicketID on the event.
func TestWorkspaceMonitor_TicketPattern_DefaultExtraction(t *testing.T) {
	ws := &WorkspaceMonitor{workspaceName: "default-pattern-ws", ticketPattern: ""}

	ext, err := ticket.NewExtractor(ws.ticketPattern)
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	got := ext.Extract("feat/PROJ-123-add-login")
	if got != "PROJ-123" {
		t.Errorf("Extract(branch) = %q, want %q", got, "PROJ-123")
	}
}

// TestWorkspaceMonitor_TicketPattern_NoMatchIsUnlinked confirms a branch with
// no extractable ticket (e.g. "main", "chore/update-readme") yields an empty
// TicketID — the "unlinked" case — without error or panic.
func TestWorkspaceMonitor_TicketPattern_NoMatchIsUnlinked(t *testing.T) {
	ws := &WorkspaceMonitor{workspaceName: "default-pattern-ws", ticketPattern: ""}

	ext, err := ticket.NewExtractor(ws.ticketPattern)
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	cases := []string{"main", "chore/update-readme"}
	for _, branch := range cases {
		if got := ext.Extract(branch); got != "" {
			t.Errorf("Extract(%q) = %q, want empty (unlinked)", branch, got)
		}
	}
}

// TestWorkspaceMonitor_TicketPattern_CustomOverridesDefault confirms a
// workspace-level custom ticketPattern (from workspaces.yaml's ticket_pattern
// field) is honoured instead of the default multi-pattern extractor.
func TestWorkspaceMonitor_TicketPattern_CustomOverridesDefault(t *testing.T) {
	ws := &WorkspaceMonitor{workspaceName: "custom-pattern-ws", ticketPattern: `(?P<ticket>DT-\d+)`}

	ext, err := ticket.NewExtractor(ws.ticketPattern)
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	// Matches the custom pattern.
	if got := ext.Extract("feat/DT-999-thing"); got != "DT-999" {
		t.Errorf("Extract with custom pattern = %q, want %q", got, "DT-999")
	}

	// A default-pattern-style ticket ID (Jira PROJ-123) must NOT match once a
	// custom pattern is set — custom patterns are exclusive, not additive.
	if got := ext.Extract("feat/PROJ-123-add-login"); got != "" {
		t.Errorf("Extract with custom pattern matched default-style ticket: got %q, want empty", got)
	}
}

// TestTriggerEvent_CarriesTicketID confirms the TriggerEvent struct (the
// payload handed from handleCommitForWorkspace to handleTrigger) has the
// TicketID field threaded through, per TASK-068 step 6.
func TestTriggerEvent_CarriesTicketID(t *testing.T) {
	event := TriggerEvent{
		Type:     TriggerTypeCommit,
		TicketID: "PROJ-123",
	}
	if event.TicketID != "PROJ-123" {
		t.Errorf("TriggerEvent.TicketID = %q, want %q", event.TicketID, "PROJ-123")
	}
}

// --- TASK-069: commit-message fallback + active-ticket fallback ---------
//
// These tests mirror the three-strategy fallback chain implemented in
// handleCommitForWorkspace: branch -> commit message -> active-ticket
// (DB lookup). The DB-backed third strategy is covered separately by
// TestGetLastTicketID in internal/db/trigger_ticket_test.go; here we
// exercise strategies 1 and 2 exactly as handleCommitForWorkspace calls them.

// TestFallbackChain_BranchMatchWinsOverMessage confirms strategy 1 (branch)
// short-circuits the chain — when the branch already yields a ticket, the
// commit message is never consulted.
func TestFallbackChain_BranchMatchWinsOverMessage(t *testing.T) {
	ext, err := ticket.NewExtractor("")
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	branch := "feat/PROJ-123-add-login"
	message := "fix bug in login AB-99"

	ticketID := ext.Extract(branch)
	if ticketID == "" {
		ticketID = ext.Extract(message)
	}

	if ticketID != "PROJ-123" {
		t.Errorf("ticketID = %q, want %q (branch match should win over message)", ticketID, "PROJ-123")
	}
}

// TestFallbackChain_MessageScanRunsWhenBranchEmpty confirms strategy 2: when
// branch extraction returns "", the commit message is scanned next and its
// match is used as the ticket ID.
func TestFallbackChain_MessageScanRunsWhenBranchEmpty(t *testing.T) {
	ext, err := ticket.NewExtractor("")
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	branch := "feat/no-ticket-here"
	message := "fix bug in login AB-99"

	ticketID := ext.Extract(branch)
	if ticketID != "" {
		t.Fatalf("branch extraction = %q, want empty for this test fixture", ticketID)
	}
	ticketID = ext.Extract(message)

	if ticketID != "AB-99" {
		t.Errorf("ticketID = %q, want %q (message-scan fallback)", ticketID, "AB-99")
	}
}

// TestFallbackChain_AllStrategiesFailYieldsUnlinked confirms that when branch
// and message extraction both fail (and there is no active-ticket fallback
// available), the final ticketID is "" — the unlinked case. This matches the
// acceptance criterion: a commit on branch "main" with message "chore:
// update docs" and no prior commits produces ticket_id="".
func TestFallbackChain_AllStrategiesFailYieldsUnlinked(t *testing.T) {
	ext, err := ticket.NewExtractor("")
	if err != nil {
		t.Fatalf("NewExtractor: %v", err)
	}

	branch := "main"
	message := "chore: update docs"

	ticketID := ext.Extract(branch)
	if ticketID == "" {
		ticketID = ext.Extract(message)
	}
	// No active-ticket fallback simulated here (no prior matched commits).

	if ticketID != "" {
		t.Errorf("ticketID = %q, want empty (unlinked)", ticketID)
	}
}
