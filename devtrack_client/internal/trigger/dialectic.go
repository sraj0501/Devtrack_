package trigger

import (
	"fmt"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// InferenceResult mirrors one inference dict returned by POST /dialectic/infer.
// Fields map directly to the Python response JSON shape.
type InferenceResult struct {
	ContextType   string  `json:"context_type"`  // populated from the request context_type
	Subject       string  `json:"subject"`
	InferenceText string  `json:"inference"`
	Confidence    float64 `json:"confidence"`
}

// dialecticInferRequest is the body sent to POST /dialectic/infer.
type dialecticInferRequest struct {
	InteractionType string         `json:"interaction_type"`
	ContextType     string         `json:"context_type"`
	BeforeText      string         `json:"before_text"`
	AfterText       string         `json:"after_text"`
	Metadata        map[string]any `json:"metadata"`
}

// dialecticInferResponse is the shape of the POST /dialectic/infer response.
type dialecticInferResponse struct {
	Inferences []struct {
		Subject    string  `json:"subject"`
		Inference  string  `json:"inference"`
		Confidence float64 `json:"confidence"`
	} `json:"inferences"`
}

// PostDialecticInfer calls POST /dialectic/infer with the relevant fields from a
// PendingAction. It returns a slice of InferenceResult (possibly empty) and a
// non-nil error only when the HTTP call itself fails.
//
// The caller must treat the results as best-effort: when an error is returned,
// log it and continue — never block the main flow.
//
// The context_type is derived from action.ActionType using a simple mapping.
func (c *HTTPTriggerClient) PostDialecticInfer(action db.PendingAction) ([]InferenceResult, error) {
	contextType := actionTypeToContextType(action.ActionType)

	req := dialecticInferRequest{
		InteractionType: "commit", // queue executor fires after a successful POST
		ContextType:     contextType,
		BeforeText:      "",
		AfterText:       action.Payload,
		Metadata: map[string]any{
			"action_id": action.ID,
			"target":    action.Target,
			"workspace": action.Workspace,
			"platform":  action.Platform,
		},
	}

	var resp dialecticInferResponse
	if err := c.postWithResult("/dialectic/infer", req, &resp); err != nil {
		return nil, fmt.Errorf("PostDialecticInfer: %w", err)
	}

	results := make([]InferenceResult, 0, len(resp.Inferences))
	for _, inf := range resp.Inferences {
		results = append(results, InferenceResult{
			ContextType:   contextType,
			Subject:       inf.Subject,
			InferenceText: inf.Inference,
			Confidence:    inf.Confidence,
		})
	}
	return results, nil
}

// PostDialecticInferApproval calls POST /dialectic/infer for a TUI approve interaction.
// interaction_type is set to "approval"; before_text is empty; after_text is the payload.
func (c *HTTPTriggerClient) PostDialecticInferApproval(action db.PendingAction) ([]InferenceResult, error) {
	return c.postDialecticWithInteraction(action, "approval", "", action.Payload)
}

// PostDialecticInferRejection calls POST /dialectic/infer for a TUI reject interaction.
// interaction_type is set to "rejection"; both text fields are empty.
func (c *HTTPTriggerClient) PostDialecticInferRejection(action db.PendingAction) ([]InferenceResult, error) {
	return c.postDialecticWithInteraction(action, "rejection", action.Payload, "")
}

// postDialecticWithInteraction is the shared implementation used by all
// PostDialecticInfer* variants. It does not add the interaction_type to the
// seenIDs set — each call always fires (the caller is responsible for calling
// it exactly once per event, fire-and-forget goroutine pattern).
func (c *HTTPTriggerClient) postDialecticWithInteraction(
	action db.PendingAction,
	interactionType, beforeText, afterText string,
) ([]InferenceResult, error) {
	contextType := actionTypeToContextType(action.ActionType)

	req := dialecticInferRequest{
		InteractionType: interactionType,
		ContextType:     contextType,
		BeforeText:      beforeText,
		AfterText:       afterText,
		Metadata: map[string]any{
			"action_id": action.ID,
			"target":    action.Target,
			"workspace": action.Workspace,
			"platform":  action.Platform,
		},
	}

	var resp dialecticInferResponse
	if err := c.postWithResult("/dialectic/infer", req, &resp); err != nil {
		return nil, fmt.Errorf("postDialecticWithInteraction(%s): %w", interactionType, err)
	}

	results := make([]InferenceResult, 0, len(resp.Inferences))
	for _, inf := range resp.Inferences {
		results = append(results, InferenceResult{
			ContextType:   contextType,
			Subject:       inf.Subject,
			InferenceText: inf.Inference,
			Confidence:    inf.Confidence,
		})
	}
	return results, nil
}

// actionTypeToContextType maps a PendingAction.ActionType to the context_type
// expected by the Python dialectic_reasoner.
func actionTypeToContextType(actionType string) string {
	switch actionType {
	case "post_comment":
		return "comment"
	case "state_transition":
		return "ticket_mapping"
	case "eod_report":
		return "report"
	case "create_task", "update_task":
		return "task"
	default:
		return "commit"
	}
}
