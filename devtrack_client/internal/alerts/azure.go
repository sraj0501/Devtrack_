package alerts

import (
	"fmt"
	"log"
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
