package alerts

import cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"

// NotificationFilter holds the ALERT_NOTIFY_* event routing config.
type NotificationFilter struct {
	Assigned        bool
	Comments        bool
	StatusChanges   bool
	ReviewRequested bool
}

// FilterFromConfig reads the four ALERT_NOTIFY_* env flags.
func FilterFromConfig() NotificationFilter {
	return NotificationFilter{
		Assigned:        cfg.IsAlertNotifyAssigned(),
		Comments:        cfg.IsAlertNotifyComments(),
		StatusChanges:   cfg.IsAlertNotifyStatusChanges(),
		ReviewRequested: cfg.IsAlertNotifyReviewRequested(),
	}
}

// ShouldNotify returns true when the given event type passes the filter.
func (f NotificationFilter) ShouldNotify(eventType string) bool {
	switch eventType {
	case "assigned":
		return f.Assigned
	case "comment", "mention":
		return f.Comments
	case "status_change", "state_change":
		return f.StatusChanges
	case "review_requested":
		return f.ReviewRequested
	default:
		return true
	}
}
