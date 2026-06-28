package alerts

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/github"
	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
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

// collectReviewComments polls GitHub for new review comments on PRs authored by
// the developer. Each new comment is stored in pr_review_comments (idempotent)
// and returned as a ReviewCommentEvent for classification.
//
// The workspace config determines which repos to scan. Only workspaces with
// pm_platform="github" and a non-empty pm_project (repo) are polled.
func (a *githubAlerter) collectReviewComments(database *db.Database) []ReviewCommentEvent {
	client, err := github.NewClient("")
	if err != nil {
		log.Printf("alerts/github/review: build client: %v", err)
		return nil
	}

	// Determine the developer's GitHub login.
	authUser, err := client.GetAuthenticatedUser()
	if err != nil {
		log.Printf("alerts/github/review: get authenticated user: %v", err)
		return nil
	}
	devLogin := authUser.Login

	// Load workspace config to find GitHub repos.
	wsCfg, err := cfg.LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		log.Printf("alerts/github/review: load workspaces: %v", err)
		return nil
	}

	var events []ReviewCommentEvent

	for _, ws := range wsCfg.Workspaces {
		if ws.PMPlatform != "github" || !ws.Enabled {
			continue
		}
		repo := ws.PMProject
		if repo == "" {
			// Try pm_org/name derived repo
			if ws.PMOrg != "" && ws.Name != "" {
				repo = ws.PMOrg + "/" + ws.Name
			}
		}
		if repo == "" {
			continue
		}

		evs := a.collectReviewCommentsForRepo(database, client, repo, ws.Name, devLogin)
		events = append(events, evs...)
	}

	return events
}

// collectReviewCommentsForRepo fetches and processes review comments for a single
// GitHub repo. Comments already stored in pr_review_comments are skipped.
func (a *githubAlerter) collectReviewCommentsForRepo(
	database *db.Database,
	client *github.Client,
	repo, wsName, devLogin string,
) []ReviewCommentEvent {
	// List all open PRs in this repo authored by the developer.
	prs, err := client.ListOpenPRsAuthored(repo, devLogin)
	if err != nil {
		log.Printf("alerts/github/review[%s]: list PRs: %v", repo, err)
		return nil
	}

	var events []ReviewCommentEvent
	for _, pr := range prs {
		prID := fmt.Sprintf("%d", pr.Number)

		// Inline review comments (file-level).
		inlineComments, err := client.ListPRReviewComments(repo, pr.Number)
		if err != nil {
			log.Printf("alerts/github/review[%s#%d]: list inline comments: %v", repo, pr.Number, err)
		}

		// Top-level (issue) comments on the PR thread.
		issueComments, err := client.ListPRIssueComments(repo, pr.Number)
		if err != nil {
			log.Printf("alerts/github/review[%s#%d]: list issue comments: %v", repo, pr.Number, err)
		}

		for _, cmt := range append(inlineComments, issueComments...) {
			// Skip comments left by the developer themselves.
			if cmt.User.Login == devLogin {
				continue
			}

			commentID := fmt.Sprintf("%d", cmt.ID)

			// Idempotency check: skip if already stored.
			existing, err := database.GetPRReviewComment("github", commentID)
			if err != nil {
				log.Printf("alerts/github/review: db check comment %s: %v", commentID, err)
				continue
			}
			if existing != nil {
				continue // already processed
			}

			// Store the new comment.
			dbComment := db.PRReviewComment{
				Platform:    "github",
				CommentID:   commentID,
				PRID:        prID,
				Workspace:   wsName,
				Status:      "new",
				CommentBody: cmt.Body,
			}
			if err := database.InsertPRReviewComment(dbComment); err != nil {
				log.Printf("alerts/github/review: insert comment %s: %v", commentID, err)
				continue
			}

			events = append(events, ReviewCommentEvent{
				Platform:    "github",
				Workspace:   wsName,
				PRID:        prID,
				PRTitle:     pr.Title,
				CommentID:   commentID,
				CommentBody: cmt.Body,
				Reviewer:    cmt.User.Login,
				CommentURL:  cmt.HTMLURL,
				DetectedAt:  time.Now(),
			})
		}
	}
	return events
}

// IsPRApproved returns true if any reviewer has submitted an APPROVED review on
// the given PR. It looks up the repo via the workspace name in workspaces.yaml.
func (a *githubAlerter) IsPRApproved(prID, workspace string) (bool, error) {
	client, err := github.NewClient("")
	if err != nil {
		return false, fmt.Errorf("alerts/github/IsPRApproved: build client: %w", err)
	}

	// Load workspace config to find the repo for this workspace.
	wsCfg, err := cfg.LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		return false, fmt.Errorf("alerts/github/IsPRApproved: load workspaces: %w", err)
	}

	var repo string
	for _, ws := range wsCfg.Workspaces {
		if ws.Name == workspace && ws.PMPlatform == "github" {
			repo = ws.PMProject
			if repo == "" && ws.PMOrg != "" && ws.Name != "" {
				repo = ws.PMOrg + "/" + ws.Name
			}
			break
		}
	}
	if repo == "" {
		return false, fmt.Errorf("alerts/github/IsPRApproved: no github repo found for workspace %q", workspace)
	}

	prNumber, err := strconv.Atoi(prID)
	if err != nil {
		return false, fmt.Errorf("alerts/github/IsPRApproved: invalid prID %q: %w", prID, err)
	}

	reviews, err := client.ListPRReviews(repo, prNumber)
	if err != nil {
		return false, fmt.Errorf("alerts/github/IsPRApproved: list reviews for %s#%d: %w", repo, prNumber, err)
	}

	for _, r := range reviews {
		if r.State == "APPROVED" {
			return true, nil
		}
	}
	return false, nil
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
