package trigger

import (
	"fmt"
	"net/url"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// QueueHistory reads the server's authoritative queue; IDs remain server-owned.
func (c *HTTPTriggerClient) QueueHistory(actionType string, hours, limit int) ([]db.PendingAction, error) {
	var actions []db.PendingAction
	for offset := 0; ; offset += limit {
		page, err := c.queueHistoryPage(actionType, hours, limit, offset)
		if err != nil {
			return nil, err
		}
		actions = append(actions, page...)
		if limit < 1000 || len(page) < limit {
			return actions, nil
		}
	}
}

func (c *HTTPTriggerClient) queueHistoryPage(actionType string, hours, limit, offset int) ([]db.PendingAction, error) {
	var resp QueuePendingResponse
	path := fmt.Sprintf("/queue/history?action_type=%s&hours=%d&limit=%d&offset=%d", url.QueryEscape(actionType), hours, limit, offset)
	if err := c.getWithResult(path, &resp); err != nil {
		return nil, err
	}
	actions := make([]db.PendingAction, 0, len(resp.Actions))
	for _, a := range resp.Actions {
		created, err := queueTime(a.CreatedAt)
		if err != nil {
			return nil, err
		}
		expires, err := queueTime(a.ExpiresAt)
		if err != nil {
			return nil, err
		}
		var acted *time.Time
		if a.ActedAt != nil {
			t, err := queueTime(*a.ActedAt)
			if err != nil {
				return nil, err
			}
			acted = &t
		}
		actions = append(actions, db.PendingAction{ID: a.ID, ActionType: a.ActionType, Target: a.Target,
			Platform: a.Platform, Workspace: a.Workspace, Payload: a.Payload, Confidence: a.Confidence,
			Status: a.Status, CreatedAt: created, ExpiresAt: expires, ActedAt: acted, ActedBy: a.ActedBy, Error: a.Error})
	}
	return actions, nil
}

func queueTime(value string) (time.Time, error) {
	// Older server rows use timezone-less ISO strings, always UTC.
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid server queue timestamp %q", value)
}

func (c *HTTPTriggerClient) ReviewQueueAction(id int64, operation string, payload any) error {
	var resp map[string]any
	return c.postWithResult("/queue/review", map[string]any{"action_id": id, "operation": operation, "payload": payload}, &resp)
}
