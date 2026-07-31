package trigger

// Trigger payload data structures shared between the HTTP trigger client
// (this package) and the IPC server (internal/infra). They are the JSON
// payloads POSTed to the Python server's /trigger/* endpoints.

// CommitTriggerData contains information about a Git commit.
type CommitTriggerData struct {
	RepoPath      string   `json:"repo_path"`
	CommitHash    string   `json:"commit_hash"`
	CommitMessage string   `json:"commit_message"`
	Author        string   `json:"author"`
	Timestamp     string   `json:"timestamp"`
	FilesChanged  []string `json:"files_changed"`
	Branch        string   `json:"branch"`
	TicketID      string   `json:"ticket_id,omitempty"`
	// IsFirstCommitForTicket is true when CountTicketCommits returned 0 BEFORE
	// InsertTrigger was called for this commit — meaning this is the first linked
	// commit for this ticket in this repo. Used by Python (TASK-073) to decide
	// whether to stage a state_transition ("To Do → In Progress") queue action.
	// Omitted from JSON when false (zero value) to keep the payload lean.
	IsFirstCommitForTicket bool `json:"is_first_commit_for_ticket,omitempty"`
	// IsMergeToDefault is true when this commit is a merge commit that landed on
	// the repository's default branch. Used by Python (TASK-126) to stage a
	// state_transition to the platform's done state ("merged to main → Done").
	IsMergeToDefault bool `json:"is_merge_to_default,omitempty"`
	// TicketConfidence reflects the extraction strategy that produced TicketID
	// (TASK-128): 0.95 branch name, 0.85 commit message, 0.60 active-ticket
	// fallback. Python uses it as the confidence for staged queue actions.
	TicketConfidence float64 `json:"ticket_confidence,omitempty"`
	// Workspace routing fields (omitempty — zero value = fall back to priority chain)
	WorkspaceName string `json:"workspace_name,omitempty"`
	PMPlatform    string `json:"pm_platform,omitempty"`
	PMProject     string `json:"pm_project,omitempty"`
	// Per-workspace PM settings (omitempty — zero value = no override)
	PMAssignee      string `json:"pm_assignee,omitempty"`
	PMIterationPath string `json:"pm_iteration_path,omitempty"`
	PMAreaPath      string `json:"pm_area_path,omitempty"`
	PMMilestone     int    `json:"pm_milestone,omitempty"`
	// PMInProgressLabel is the GitHub/GitLab in-progress label convention
	// (TASK-129); "" = default "in-progress", "none" = disabled.
	PMInProgressLabel string `json:"pm_in_progress_label,omitempty"`
}

// TimerTriggerData contains information about a scheduled trigger.
type TimerTriggerData struct {
	Timestamp    string `json:"timestamp"`
	IntervalMins int    `json:"interval_mins"`
	TriggerCount int    `json:"trigger_count"`
	// Workspace routing fields (omitempty — populated from most-recently-active workspace)
	WorkspaceName string `json:"workspace_name,omitempty"`
	PMPlatform    string `json:"pm_platform,omitempty"`
	PMProject     string `json:"pm_project,omitempty"`
	// Per-workspace PM settings (omitempty — zero value = no override)
	PMAssignee      string `json:"pm_assignee,omitempty"`
	PMIterationPath string `json:"pm_iteration_path,omitempty"`
	PMAreaPath      string `json:"pm_area_path,omitempty"`
	PMMilestone     int    `json:"pm_milestone,omitempty"`
}

// TaskUpdateData contains information about a task update.
type TaskUpdateData struct {
	Project         string `json:"project"`
	TicketID        string `json:"ticket_id"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	TimeSpent       string `json:"time_spent"`
	Synced          bool   `json:"synced"`
	AzureWorkItemID int    `json:"azure_work_item_id,omitempty"`
	SyncedPlatform  string `json:"synced_platform,omitempty"`
}
