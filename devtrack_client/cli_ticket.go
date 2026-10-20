package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
)

func (cli *CLI) handleTicket() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: devtrack ticket convention | check [branch] | link <commit> <ticket-ref>")
	}
	switch os.Args[2] {
	case "convention":
		return printTicketConvention()
	case "check":
		branch := ""
		if len(os.Args) > 3 {
			branch = os.Args[3]
		} else {
			out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
			if err != nil {
				return fmt.Errorf("read current branch: %w", err)
			}
			branch = strings.TrimSpace(string(out))
		}
		return checkTicketBranch(branch)
	case "link":
		if len(os.Args) != 5 {
			return fmt.Errorf("usage: devtrack ticket link <commit> <ticket-ref>")
		}
		return linkTicketMapping(os.Args[3], os.Args[4])
	default:
		return fmt.Errorf("unknown ticket command %q", os.Args[2])
	}
}

func currentTicketWorkspace() (*config.WorkspaceConfig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ws, err := config.ResolveWorkspaceForPath(cwd)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, fmt.Errorf("current directory is not inside an enabled DevTrack workspace")
	}
	return ws, nil
}

func resolverForWorkspace(ws *config.WorkspaceConfig) (*ticket.Resolver, error) {
	return ticket.NewResolver(ticket.ResolverConfig{
		TicketKey: ws.TicketKey, Kinds: ws.TicketKinds, BranchPattern: ws.TicketPattern,
	})
}

func printTicketConvention() error {
	ws, err := currentTicketWorkspace()
	if err != nil {
		return err
	}
	if _, err := resolverForWorkspace(ws); err != nil {
		return err
	}
	key := ws.TicketKey
	if key == "" {
		key = "<UPPERCASE-KEY>"
	}
	fmt.Printf("Workspace: %s\n", ws.Name)
	if ws.TicketPattern != "" {
		fmt.Printf("Branch pattern: %s\n", ws.TicketPattern)
	} else {
		fmt.Printf("Branch convention: <kind>/%s-<number>-<slug>\n", key)
		fmt.Printf("Kinds: %s\n", strings.Join(ticketKinds(ws), ", "))
	}
	fmt.Println("Precedence: canonical branch > explicit prefix/trailer > active work ticket > unlinked")
	return nil
}

func ticketKinds(ws *config.WorkspaceConfig) []string {
	if len(ws.TicketKinds) > 0 {
		return ws.TicketKinds
	}
	return ticket.DefaultKinds
}

func checkTicketBranch(branch string) error {
	ws, err := currentTicketWorkspace()
	if err != nil {
		return err
	}
	resolver, err := resolverForWorkspace(ws)
	if err != nil {
		return err
	}
	result := resolver.Resolve(ticket.ResolveInput{Branch: branch})
	if result.TicketID == "" {
		fmt.Printf("Branch %q is nonconforming; commits remain unlinked and Git is not blocked.\n", branch)
		return nil
	}
	fmt.Printf("Branch %q maps to %s (source=%s, confidence=%.2f).\n",
		branch, result.TicketID, result.Source, result.Confidence)
	return nil
}

func linkTicketMapping(commitRef, replacementRef string) error {
	database, err := NewDatabase()
	if err != nil {
		return err
	}
	defer database.Close()
	record, err := database.FindCommitTrigger(commitRef)
	if err != nil {
		return err
	}
	ws, err := config.ResolveWorkspaceForPath(record.RepoPath)
	if err != nil {
		return fmt.Errorf("resolve workspace for observed commit: %w", err)
	}
	if ws == nil {
		return fmt.Errorf("observed commit is not inside an enabled DevTrack workspace")
	}
	resolver, err := resolverForWorkspace(ws)
	if err != nil {
		return err
	}
	validated := resolver.Resolve(ticket.ResolveInput{ActiveTicket: replacementRef})
	if validated.TicketID != replacementRef || validated.Source != ticket.SourceActiveTicket {
		return fmt.Errorf("ticket reference %q does not match workspace %q", replacementRef, ws.Name)
	}
	_, effective, err := database.GetEffectiveTicketMapping(record.ID)
	if err != nil {
		return err
	}
	id, err := database.InsertTicketMappingCorrection(db.TicketMappingCorrection{
		TriggerID: record.ID, PreviousRef: effective, ReplacementRef: replacementRef,
		Channel: "cli", Actor: "user", Reason: "explicit ticket link",
	})
	if err != nil {
		return err
	}
	fmt.Printf("Linked commit %s to %s (correction %d; original evidence preserved).\n",
		record.CommitHash, replacementRef, id)
	return nil
}

func printTicketMappingHealth() {
	cfg, err := LoadWorkspacesConfig()
	if err != nil {
		fmt.Printf("Ticket convention: invalid configuration: %v\n\n", err)
		return
	}
	fmt.Println("Ticket convention:")
	for _, ws := range cfg.GetEnabledWorkspaces() {
		key := ws.TicketKey
		if key == "" {
			key = "any uppercase key"
		}
		fmt.Printf("  %-20s key=%s\n", ws.Name, key)
	}
	database, err := NewDatabase()
	if err == nil {
		defer database.Close()
		unlinked, conflicts, statsErr := database.TicketMappingHealth("", ticketExtractionWindow)
		if statsErr == nil {
			fmt.Printf("  Recent mappings: %d unlinked, %d conflicted\n", unlinked, conflicts)
		}
	}
	fmt.Println()
}
