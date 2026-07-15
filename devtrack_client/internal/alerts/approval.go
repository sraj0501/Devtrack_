package alerts

import "strings"

// ApprovalChecker exposes the platform alerters' IsPRApproved to other packages
// (the PR fix loop). It satisfies reviewer.PRApprovalChecker.
type ApprovalChecker struct {
	platform string
}

// NewApprovalChecker returns a checker for the given platform, or nil when the
// platform cannot report PR approval state (e.g. GitLab — no MR approval API
// support in the connector yet). Callers must assign a nil result to a nil
// interface value, never a typed-nil pointer.
func NewApprovalChecker(platform string) *ApprovalChecker {
	switch strings.ToLower(platform) {
	case "github", "azure":
		return &ApprovalChecker{platform: strings.ToLower(platform)}
	default:
		return nil
	}
}

func (c *ApprovalChecker) IsPRApproved(prID, workspace string) (bool, error) {
	if c.platform == "azure" {
		return (&azureAlerter{}).IsPRApproved(prID, workspace)
	}
	return (&githubAlerter{}).IsPRApproved(prID, workspace)
}
