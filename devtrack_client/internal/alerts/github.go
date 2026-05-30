package alerts

import (
	"log"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/github"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

type githubAlerter struct {
	filter NotificationFilter
}

// collect polls the GitHub Notifications API for events since the last check
// and returns candidate NotificationRecords to be inserted by the poller.
func (a *githubAlerter) collect(database *db.Database, userID string) []db.NotificationRecord {
	client, err := github.NewClient("")
	if err != nil {
		log.Printf("alerts/github: %v", err)
		return nil
	}

	since, _, err := database.GetAlertLastChecked(userID, "github")
	if err != nil {
		log.Printf("alerts/github: load last_checked: %v", err)
	}

	notifs, err := client.ListNotificationsSince(since)
	if err != nil {
		log.Printf("alerts/github: poll: %v", err)
		return nil
	}

	if err := database.SetAlertLastChecked(userID, "github", time.Now()); err != nil {
		log.Printf("alerts/github: save last_checked: %v", err)
	}

	var records []db.NotificationRecord
	for _, n := range notifs {
		eventType := mapGitHubReason(n.Reason)
		if !a.filter.ShouldNotify(eventType) {
			continue
		}
		records = append(records, db.NotificationRecord{
			Source:    "github",
			EventType: eventType,
			TicketID:  n.Repository.FullName + "#" + n.ID,
			Title:     n.Subject.Title,
			Body:      n.Reason,
			URL:       apiURLToWebURL(n.Subject.URL),
		})
	}
	return records
}

func mapGitHubReason(reason string) string {
	switch reason {
	case "assigned":
		return "assigned"
	case "review_requested":
		return "review_requested"
	case "comment", "mention":
		return "comment"
	case "state_change":
		return "status_change"
	default:
		return reason
	}
}

// apiURLToWebURL converts a GitHub API URL to an HTML URL.
// https://api.github.com/repos/owner/repo/issues/123 → https://github.com/owner/repo/issues/123
func apiURLToWebURL(apiURL string) string {
	return strings.Replace(apiURL, "https://api.github.com/repos/", "https://github.com/", 1)
}
