package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	azureconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/azure"
	githubconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/github"
	gitlabconn "github.com/sraj0501/Devtrack_/devtrack_client/connectors/gitlab"
)

// handleAzureCheck verifies Azure DevOps config and connectivity
func (cli *CLI) handleAzureCheck() error {
	return azureconn.Check()
}

// handleAzureList lists work items assigned to the user
func (cli *CLI) handleAzureList() error {
	items, err := azureconn.ListWorkItems()
	if err != nil {
		return fmt.Errorf("azure list: %w", err)
	}
	if len(items) == 0 {
		fmt.Println("No open work items assigned to you.")
		return nil
	}
	for _, item := range items {
		fmt.Printf("#%d  [%s] (%s)  %s\n  %s\n\n",
			item.ID, item.State(), item.WorkItemType(), item.Title(), item.WebURL)
	}
	return nil
}

// handleAzureSync runs a manual sync with Azure DevOps and stores results in SQLite.
func (cli *CLI) handleAzureSync() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("azure sync: open database: %w", err)
	}
	defer db.Close()
	return azureconn.Sync(db.DB())
}

// handleAzureView shows details for a specific Azure DevOps work item
func (cli *CLI) handleAzureView() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devtrack azure-view <work-item-id>")
		return fmt.Errorf("missing work item ID")
	}
	id, err := strconv.Atoi(os.Args[2])
	if err != nil {
		return fmt.Errorf("azure-view: invalid work item ID %q: %w", os.Args[2], err)
	}
	item, err := azureconn.ViewWorkItem(id)
	if err != nil {
		return fmt.Errorf("azure view: %w", err)
	}
	fmt.Print(azureconn.FormatWorkItem(item))
	return nil
}

// handleGitLabCheck verifies GitLab config and connectivity
func (cli *CLI) handleGitLabCheck() error {
	return gitlabconn.Check()
}

// handleGitLabList lists GitLab issues assigned to the user
func (cli *CLI) handleGitLabList() error {
	issues, err := gitlabconn.ListIssues()
	if err != nil {
		return fmt.Errorf("gitlab list: %w", err)
	}
	if len(issues) == 0 {
		fmt.Println("No open issues assigned to you.")
		return nil
	}
	for _, iss := range issues {
		fmt.Printf("#%d  [%s]  %s\n  %s\n\n", iss.IID, iss.State, iss.Title, iss.URL)
	}
	return nil
}

// handleGitLabSync runs a manual sync with GitLab and stores results in SQLite.
func (cli *CLI) handleGitLabSync() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("gitlab sync: open database: %w", err)
	}
	defer db.Close()
	return gitlabconn.Sync(db.DB())
}

// handleGitLabView shows details for a specific GitLab issue.
// Usage: devtrack gitlab-view <project_id_or_path> <issue_iid>
func (cli *CLI) handleGitLabView() error {
	if len(os.Args) < 4 {
		fmt.Println("Usage: devtrack gitlab-view <project_id_or_path> <issue_iid>")
		return fmt.Errorf("missing project_id and/or issue_iid")
	}
	projectPath := os.Args[2]
	iid, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return fmt.Errorf("gitlab-view: invalid issue IID %q: %w", os.Args[3], err)
	}
	issue, err := gitlabconn.ViewIssue(projectPath, iid)
	if err != nil {
		return fmt.Errorf("gitlab view: %w", err)
	}
	fmt.Print(gitlabconn.FormatIssue(issue))
	return nil
}

// handleGitHubCheck verifies GitHub config and connectivity
func (cli *CLI) handleGitHubCheck() error {
	return githubconn.Check()
}

// handleGitHubList lists GitHub issues assigned to the user
func (cli *CLI) handleGitHubList() error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is not set")
	}
	username := os.Getenv("GITHUB_USERNAME")
	issues, err := githubconn.ListIssues(token, username)
	if err != nil {
		return fmt.Errorf("github list: %w", err)
	}
	if len(issues) == 0 {
		fmt.Println("No open issues assigned to you.")
		return nil
	}
	for _, iss := range issues {
		fmt.Printf("#%d  [%s]  %s\n  %s\n  %s\n\n",
			iss.Number, iss.State, iss.Title, iss.Repo, iss.URL)
	}
	return nil
}

// handleGitHubSync runs a manual sync with GitHub and stores results in SQLite.
func (cli *CLI) handleGitHubSync() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("github sync: open database: %w", err)
	}
	defer db.Close()
	return githubconn.Sync(db.DB())
}

// handleGitHubView shows details for a specific GitHub issue.
// Usage: devtrack github-view <owner>/<repo> <issue_number>
// OR:    devtrack github-view <issue_number>  (uses GITHUB_REPO env var)
func (cli *CLI) handleGitHubView() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: devtrack github-view <owner/repo> <issue_number>")
		fmt.Println("   or: devtrack github-view <issue_number>  (requires GITHUB_REPO=owner/repo)")
		return fmt.Errorf("missing arguments")
	}

	var owner, repo string
	var number int
	var err error

	if len(os.Args) >= 4 {
		// devtrack github-view owner/repo 42
		parts := strings.SplitN(os.Args[2], "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("github-view: expected owner/repo, got %q", os.Args[2])
		}
		owner, repo = parts[0], parts[1]
		number, err = strconv.Atoi(os.Args[3])
	} else {
		// devtrack github-view 42  — requires GITHUB_REPO
		repoEnv := os.Getenv("GITHUB_REPO")
		if repoEnv == "" {
			return fmt.Errorf("GITHUB_REPO not set — use: devtrack github-view <owner/repo> <number>")
		}
		parts := strings.SplitN(repoEnv, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("GITHUB_REPO must be in owner/repo format, got %q", repoEnv)
		}
		owner, repo = parts[0], parts[1]
		number, err = strconv.Atoi(os.Args[2])
	}
	if err != nil {
		return fmt.Errorf("github-view: invalid issue number: %w", err)
	}

	issue, err := githubconn.ViewIssue(owner, repo, number)
	if err != nil {
		return fmt.Errorf("github view: %w", err)
	}
	fmt.Print(githubconn.FormatIssue(issue))
	return nil
}
