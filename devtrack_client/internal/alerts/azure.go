package alerts

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/azure"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

type azureAlerter struct {
	filter NotificationFilter
}

// collect iterates all Azure workspaces and returns candidate notifications
// for work items assigned to @Me that changed since the last check.
func (a *azureAlerter) collect(database *db.Database, userID string) []db.NotificationRecord {
	wsCfg, err := config.LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		return nil
	}

	var all []db.NotificationRecord
	for _, ws := range wsCfg.Workspaces {
		if ws.PMPlatform != "azure" || !ws.Enabled {
			continue
		}
		recs := a.collectWorkspace(database, userID, ws)
		all = append(all, recs...)
	}
	return all
}

// IsPRApproved returns true if any reviewer has cast an Approved vote (vote >= 10)
// on the given pull request in Azure DevOps. It loads workspaces config, finds the
// Azure workspace matching the workspace param, and calls the real ADO Pull Requests API.
func (a *azureAlerter) IsPRApproved(prID, workspace string) (bool, error) {
	wsCfg, err := config.LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		return false, fmt.Errorf("alerts/azure/IsPRApproved: load workspaces: %w", err)
	}

	var matched *config.WorkspaceConfig
	for i := range wsCfg.Workspaces {
		ws := &wsCfg.Workspaces[i]
		if ws.Name == workspace && ws.PMPlatform == "azure" {
			matched = ws
			break
		}
	}
	if matched == nil {
		return false, fmt.Errorf("alerts/azure/IsPRApproved: no azure workspace found for %q", workspace)
	}

	client, err := azure.NewClient(matched.PMOrg, matched.PMProject, matched.PMAPIURL)
	if err != nil {
		return false, fmt.Errorf("alerts/azure/IsPRApproved: build client: %w", err)
	}

	prNumber, err := strconv.Atoi(prID)
	if err != nil {
		return false, fmt.Errorf("alerts/azure/IsPRApproved: invalid prID %q: %w", prID, err)
	}

	reviewers, err := client.ListPRReviewers(prNumber)
	if err != nil {
		return false, fmt.Errorf("alerts/azure/IsPRApproved: list reviewers for PR %d: %w", prNumber, err)
	}

	for _, r := range reviewers {
		if r.Vote >= 10 {
			return true, nil
		}
	}
	return false, nil
}

func (a *azureAlerter) collectWorkspace(database *db.Database, userID string, ws config.WorkspaceConfig) []db.NotificationRecord {
	client, err := azure.NewClient(ws.PMOrg, ws.PMProject, ws.PMAPIURL)
	if err != nil {
		log.Printf("alerts/azure[%s]: %v", ws.Name, err)
		return nil
	}

	source := "azure:" + ws.Name
	since, _, err := database.GetAlertLastChecked(userID, source)
	if err != nil {
		log.Printf("alerts/azure[%s]: load last_checked: %v", ws.Name, err)
	}

	items, err := client.ListWorkItemsChangedAfter(since)
	if err != nil {
		log.Printf("alerts/azure[%s]: poll: %v", ws.Name, err)
		return nil
	}

	if err := database.SetAlertLastChecked(userID, source, time.Now()); err != nil {
		log.Printf("alerts/azure[%s]: save last_checked: %v", ws.Name, err)
	}

	var records []db.NotificationRecord
	for _, item := range items {
		if !a.filter.ShouldNotify("assigned") {
			continue
		}
		records = append(records, db.NotificationRecord{
			Source:    "azure",
			EventType: "assigned",
			TicketID:  fmt.Sprintf("%d", item.ID),
			Title:     item.Title(),
			Body:      item.State(),
			URL:       item.WebURL,
		})
	}
	return records
}
