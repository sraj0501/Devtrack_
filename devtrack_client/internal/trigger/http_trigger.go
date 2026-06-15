package trigger

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// HTTPTriggerClient sends trigger events to the Python backend via HTTPS POST.
// Always used — both managed and external modes now communicate over HTTP.
// When TLS is enabled (default) the client pins the self-signed cert generated
// at daemon startup so no InsecureSkipVerify is needed.
type HTTPTriggerClient struct {
	serverURL  string
	apiKey     string
	httpClient *http.Client
}

// NewHTTPTriggerClient builds a client pointing at GetServerURL().
// If TLS is enabled it loads the daemon-generated cert from GetTLSCertPath()
// and uses it as the sole trusted root (cert-pinning).
func NewHTTPTriggerClient() *HTTPTriggerClient {
	transport := &http.Transport{}

	if config.IsLocalTLS() {
		// Cert-pin the locally generated self-signed cert (managed / external-local mode).
		// In cloud mode we skip pinning and use system CA roots instead.
		pool, err := LoadTLSCertPool(config.GetTLSCertPath())
		if err != nil {
			log.Printf("Warning: could not load TLS cert for HTTP client (%v) — falling back to system roots", err)
		} else {
			transport.TLSClientConfig = &tls.Config{
				RootCAs:    pool,
				MinVersion: tls.VersionTLS12,
			}
		}
	}

	return &HTTPTriggerClient{
		serverURL: config.GetServerURL(),
		apiKey:    config.GetCloudAPIKey(),
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// Ping checks whether the Python server is reachable and healthy.
func (c *HTTPTriggerClient) Ping() bool {
	if c.serverURL == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", c.serverURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// SendCommitTrigger POSTs a commit trigger payload to POST /trigger/commit.
func (c *HTTPTriggerClient) SendCommitTrigger(data CommitTriggerData) error {
	return c.post("/trigger/commit", data)
}

// SendTimerTrigger POSTs a timer trigger payload to POST /trigger/timer.
func (c *HTTPTriggerClient) SendTimerTrigger(data TimerTriggerData) error {
	return c.post("/trigger/timer", data)
}

// SendWorkspaceReload POSTs a workspace-reload notification to Python.
func (c *HTTPTriggerClient) SendWorkspaceReload() error {
	return c.post("/trigger/workspace_reload", map[string]string{"source": "cli"})
}

// SendShutdown notifies Python to perform a graceful shutdown.
func (c *HTTPTriggerClient) SendShutdown() error {
	return c.post("/trigger/shutdown", map[string]string{})
}

// SendPing checks liveness via /trigger/ping.
func (c *HTTPTriggerClient) SendPing() error {
	return c.post("/trigger/ping", map[string]string{})
}

// HeartbeatWorkspace is one monitored workspace entry sent in a heartbeat.
type HeartbeatWorkspace struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

// HeartbeatPayload is the body sent to POST /trigger/client/heartbeat.
type HeartbeatPayload struct {
	ClientID   string               `json:"client_id"`
	Version    string               `json:"version"`
	TLSEnabled bool                 `json:"tls_enabled"`
	Workspaces []HeartbeatWorkspace `json:"workspaces"`
}

// SendHeartbeat registers this client with the server and reports which
// workspaces it is monitoring.  The server uses this to populate the
// "Connected Clients" panel in the admin dashboard.
func (c *HTTPTriggerClient) SendHeartbeat(payload HeartbeatPayload) error {
	return c.post("/trigger/client/heartbeat", payload)
}

// SendWorkSessionStart notifies Python that a work session has started.
func (c *HTTPTriggerClient) SendWorkSessionStart(sessionID int64, ticketRef string) error {
	return c.post("/trigger/work_session_start", map[string]any{
		"session_id": sessionID,
		"ticket_ref": ticketRef,
	})
}

// SendWorkSessionStop notifies Python that a work session has ended.
func (c *HTTPTriggerClient) SendWorkSessionStop(sessionID int64) error {
	return c.post("/trigger/work_session_stop", map[string]any{
		"session_id": sessionID,
	})
}

// SlimTicket is the minimal ticket representation sent to the Python server
// for fuzzy/semantic matching. Body/description is truncated to 500 chars to
// keep payloads small — Python only needs enough text for embedding.
type SlimTicket struct {
	ID          string `json:"id"`           // composite: "github:owner/repo#123"
	ExternalID  string `json:"external_id"`  // raw platform identifier: "123"
	Source      string `json:"source"`       // "github" | "azure" | "gitlab"
	Repo        string `json:"repo"`         // "owner/repo" (empty for Azure)
	Title       string `json:"title"`
	Description string `json:"description"`  // truncated to 500 chars
	Status      string `json:"status"`       // "open", "In Progress", etc.
	Assignee    string `json:"assignee"`
	URL         string `json:"url"`
}

// TicketSyncPayload is the body of POST /trigger/ticket_sync.
type TicketSyncPayload struct {
	Source    string       `json:"source"`     // "github" | "azure" | "gitlab"
	Workspace string       `json:"workspace"`  // workspace name from workspaces.yaml
	Force     bool         `json:"force"`      // true → Python drops+reloads, false → upsert
	SyncedAt  string       `json:"synced_at"`  // RFC3339 timestamp of this sync
	Tickets   []SlimTicket `json:"tickets"`
}

// SendTicketSync pushes a slim ticket list to POST /trigger/ticket_sync.
// Non-blocking best-effort: if Python is down the commit pipeline still works.
func (c *HTTPTriggerClient) SendTicketSync(payload TicketSyncPayload) error {
	return c.post("/trigger/ticket_sync", payload)
}

// PlanPreviewRequest is the payload sent to POST /trigger/plan/preview.
type PlanPreviewRequest struct {
	Problem        string `json:"problem,omitempty"`
	Markdown       string `json:"markdown,omitempty"`
	Platform       string `json:"platform,omitempty"`
	ProjectContext string `json:"project_context,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

// PlanPreviewResponse is the response from POST /trigger/plan/preview.
type PlanPreviewResponse struct {
	Preview    string `json:"preview"`
	PlanToken  string `json:"plan_token"`
	TotalCount int    `json:"total_count"`
	EpicCount  int    `json:"epic_count"`
	StoryCount int    `json:"story_count"`
	TaskCount  int    `json:"task_count"`
	Platform   string `json:"platform"`
}

// PlanCreatedItem is one successfully created work item.
type PlanCreatedItem struct {
	Title       string `json:"title"`
	ItemType    string `json:"item_type"`
	Level       int    `json:"level"`
	PlatformID  string `json:"platform_id"`
	PlatformURL string `json:"platform_url"`
}

// PlanFailedItem is one work item that failed to be created.
type PlanFailedItem struct {
	Title string `json:"title"`
	Error string `json:"error"`
}

// PlanCreateResponse is the response from POST /trigger/plan/create.
type PlanCreateResponse struct {
	Created  []PlanCreatedItem `json:"created"`
	Failed   []PlanFailedItem  `json:"failed"`
	Progress []string          `json:"progress"`
}

// SendPlanPreview calls POST /trigger/plan/preview and returns the parsed response.
func (c *HTTPTriggerClient) SendPlanPreview(req PlanPreviewRequest) (*PlanPreviewResponse, error) {
	var resp PlanPreviewResponse
	if err := c.postWithResult("/trigger/plan/preview", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendPlanCreate calls POST /trigger/plan/create and returns the parsed response.
func (c *HTTPTriggerClient) SendPlanCreate(planToken string) (*PlanCreateResponse, error) {
	var resp PlanCreateResponse
	payload := map[string]string{"plan_token": planToken}
	if err := c.postWithResult("/trigger/plan/create", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// BoardroomRequest is the payload for POST /trigger/boardroom.
type BoardroomRequest struct {
	PlanText     string `json:"plan_text,omitempty"`
	Markdown     string `json:"markdown,omitempty"`
	OutputFormat string `json:"output_format"` // "terminal" | "markdown"
}

// BoardroomResponse is the response from POST /trigger/boardroom.
type BoardroomResponse struct {
	Report        string   `json:"report"`
	Verdict       string   `json:"verdict"`
	VerdictSummary string  `json:"verdict_summary"`
	Approve       int      `json:"approve"`
	Revise        int      `json:"revise"`
	Reject        int      `json:"reject"`
	Pros          []string `json:"pros"`
	Cons          []string `json:"cons"`
}

// SendBoardroom calls POST /trigger/boardroom and returns the parsed response.
// Uses a longer timeout (180s) since 7 parallel LLM calls + synthesis can be slow.
func (c *HTTPTriggerClient) SendBoardroom(req BoardroomRequest) (*BoardroomResponse, error) {
	// Build a client with a longer timeout for boardroom sessions.
	longClient := *c
	longClient.httpClient = &http.Client{
		Timeout:   180 * time.Second,
		Transport: c.httpClient.Transport,
	}
	var resp BoardroomResponse
	if err := longClient.postWithResult("/trigger/boardroom", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// BoardroomHistoryEntry is one message in the interactive boardroom conversation.
type BoardroomHistoryEntry struct {
	Role        string `json:"role"`                   // "user" | "persona" | "system"
	Content     string `json:"content"`
	PersonaID   string `json:"persona_id,omitempty"`
	PersonaName string `json:"persona_name,omitempty"`
}

// BoardroomChatRequest is the payload for POST /trigger/boardroom/chat.
type BoardroomChatRequest struct {
	PlanText    string                   `json:"plan_text"`
	History     []BoardroomHistoryEntry  `json:"history"`
	UserMessage string                   `json:"user_message,omitempty"`
	AddressedTo string                   `json:"addressed_to,omitempty"`
	FinalSay    string                   `json:"final_say,omitempty"`
}

// BoardroomPersonaResponse is one persona's reply in a chat turn.
type BoardroomPersonaResponse struct {
	PersonaID   string `json:"persona_id"`
	PersonaName string `json:"persona_name"`
	Role        string `json:"role"`
	Content     string `json:"content"`
}

// BoardroomChatResponse is the response from POST /trigger/boardroom/chat.
type BoardroomChatResponse struct {
	Responses      []BoardroomPersonaResponse `json:"responses"`
	UpdatedHistory []BoardroomHistoryEntry    `json:"updated_history"`
	SessionClosed  bool                       `json:"session_closed"`
	ClosingSummary string                     `json:"closing_summary"`
}

// SendBoardroomChat sends one interactive chat turn to the boardroom.
func (c *HTTPTriggerClient) SendBoardroomChat(req BoardroomChatRequest) (*BoardroomChatResponse, error) {
	longClient := *c
	longClient.httpClient = &http.Client{
		Timeout:   120 * time.Second,
		Transport: c.httpClient.Transport,
	}
	var resp BoardroomChatResponse
	if err := longClient.postWithResult("/trigger/boardroom/chat", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// postWithResult POSTs payload as JSON and decodes the response body into dest.
func (c *HTTPTriggerClient) postWithResult(path string, payload any, dest any) error {
	if c.serverURL == "" {
		return fmt.Errorf("DEVTRACK_SERVER_URL is not set")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.serverURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-DevTrack-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s%s: %w", c.serverURL, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to extract a detail message from FastAPI error shape.
		var errResp struct {
			Detail string `json:"detail"`
		}
		if jsonErr := json.Unmarshal(respBody, &errResp); jsonErr == nil && errResp.Detail != "" {
			return fmt.Errorf("server error (HTTP %d): %s", resp.StatusCode, errResp.Detail)
		}
		return fmt.Errorf("server returned HTTP %d for %s", resp.StatusCode, path)
	}

	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	log.Printf("✓ Request sent via HTTP → %s%s (%d)", c.serverURL, path, resp.StatusCode)
	return nil
}

func (c *HTTPTriggerClient) post(path string, payload any) error {
	if c.serverURL == "" {
		return fmt.Errorf("DEVTRACK_SERVER_URL is not set")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.serverURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-DevTrack-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s%s: %w", c.serverURL, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned HTTP %d for %s", resp.StatusCode, path)
	}

	log.Printf("✓ Trigger sent via HTTP → %s%s (%d)", c.serverURL, path, resp.StatusCode)
	return nil
}

// getWithResult performs a GET and decodes the JSON response body into dest.
func (c *HTTPTriggerClient) getWithResult(path string, dest any) error {
	if c.serverURL == "" {
		return fmt.Errorf("DEVTRACK_SERVER_URL is not set")
	}
	req, err := http.NewRequest("GET", c.serverURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-DevTrack-API-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s%s: %w", c.serverURL, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct{ Detail string `json:"detail"` }
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Detail != "" {
			return fmt.Errorf("server error (HTTP %d): %s", resp.StatusCode, errResp.Detail)
		}
		return fmt.Errorf("server returned HTTP %d for %s", resp.StatusCode, path)
	}
	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// deleteWithResult performs a DELETE and decodes the JSON response body into dest.
func (c *HTTPTriggerClient) deleteWithResult(path string, dest any) error {
	if c.serverURL == "" {
		return fmt.Errorf("DEVTRACK_SERVER_URL is not set")
	}
	req, err := http.NewRequest("DELETE", c.serverURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-DevTrack-API-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s%s: %w", c.serverURL, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned HTTP %d for %s", resp.StatusCode, path)
	}
	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// NarrativeStage is one stage record within a NarrativeStory.
type NarrativeStage struct {
	Name       string   `json:"name"`
	DurationMs *float64 `json:"duration_ms"`
	Failed     bool     `json:"failed"`
}

// NarrativeStory is one completed request story from narrative.log.
type NarrativeStory struct {
	StoryID          string           `json:"story_id"`
	Name             string           `json:"name"`
	StartedAt        string           `json:"started_at"`
	CompletedAt      string           `json:"completed_at"`
	Success          bool             `json:"success"`
	DurationMs       *float64         `json:"duration_ms"`
	TotalStages      int              `json:"total_stages"`
	CompletedStages  int              `json:"completed_stages"`
	Stages           []NarrativeStage `json:"stages"`
	Failure          map[string]any   `json:"failure"`
}

// NarrativeLastFailure is the FailureOccurred event returned by /narrative/last-failure.
type NarrativeLastFailure struct {
	StoryName   string `json:"story_name"`
	StageName   string `json:"stage_name"`
	ErrorType   string `json:"error_type"`
	ErrorMsg    string `json:"error_message"`
	Timestamp   string `json:"timestamp"`
	LLMAnalysis string `json:"llm_analysis"`
}

// GetNarrativeRecent fetches the last n request stories from the server's narrative.log.
func (c *HTTPTriggerClient) GetNarrativeRecent(n int) ([]NarrativeStory, error) {
	var resp struct {
		Stories []NarrativeStory `json:"stories"`
	}
	path := fmt.Sprintf("/narrative/recent?n=%d", n)
	if err := c.getWithResult(path, &resp); err != nil {
		return nil, err
	}
	return resp.Stories, nil
}

// GetNarrativeLastFailure fetches the most recent FailureOccurred event from narrative.log.
// Returns nil, nil when no failure has occurred.
func (c *HTTPTriggerClient) GetNarrativeLastFailure() (*NarrativeLastFailure, error) {
	var f NarrativeLastFailure
	if err := c.getWithResult("/narrative/last-failure", &f); err != nil {
		return nil, err
	}
	if f.StoryName == "" && f.StageName == "" {
		return nil, nil // empty object — no failure recorded
	}
	return &f, nil
}

// textOutput is the standard response shape for commands that return text output.
type textOutput struct {
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

// postText POSTs payload and returns the "output" string from the response.
func (c *HTTPTriggerClient) postText(path string, payload any) (string, error) {
	var r textOutput
	if err := c.postWithResult(path, payload, &r); err != nil {
		return "", err
	}
	if !r.Success {
		return r.Output, fmt.Errorf("server reported failure: %s", r.Output)
	}
	return r.Output, nil
}

// getText GETs path and returns the "output" string from the response.
func (c *HTTPTriggerClient) getText(path string) (string, error) {
	var r textOutput
	if err := c.getWithResult(path, &r); err != nil {
		return "", err
	}
	if !r.Success {
		return r.Output, fmt.Errorf("server reported failure: %s", r.Output)
	}
	return r.Output, nil
}

// ── Pending actions queue methods ─────────────────────────────────────────────

// QueuePendingAction is the minimal representation of a pending action returned
// by GET /queue/pending. Only the fields the queue executor needs are included.
type QueuePendingAction struct {
	ID         int64  `json:"id"`
	ActionType string `json:"action_type"`
	Target     string `json:"target"`
	ExpiresAt  string `json:"expires_at"` // ISO 8601 string from Python
	Status     string `json:"status"`
}

// QueuePendingResponse is the shape of the GET /queue/pending response.
type QueuePendingResponse struct {
	Actions []QueuePendingAction `json:"actions"`
}

// QueueExecuteRequest is the body sent to POST /queue/execute.
type QueueExecuteRequest struct {
	ActionID int64 `json:"action_id"`
}

// QueueExecuteResponse is the shape of the POST /queue/execute response.
type QueueExecuteResponse struct {
	Status string `json:"status"` // "posted" | "failed"
	Error  string `json:"error"`  // non-empty when status == "failed"
}

// GetQueuePending fetches all pending actions from GET /queue/pending.
// The Python server returns actions with status='pending', ordered by expires_at ASC.
func (c *HTTPTriggerClient) GetQueuePending() (*QueuePendingResponse, error) {
	var resp QueuePendingResponse
	if err := c.getWithResult("/queue/pending", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ExecuteQueueAction calls POST /queue/execute for a single action.
// Python looks up the action by ID, calls _execute_pm_action(), and marks it
// posted or failed. The executor mirrors that status in the Go-side SQLite row.
func (c *HTTPTriggerClient) ExecuteQueueAction(actionID int64) (*QueueExecuteResponse, error) {
	var resp QueueExecuteResponse
	if err := c.postWithResult("/queue/execute", QueueExecuteRequest{ActionID: actionID}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── Report methods ────────────────────────────────────────────────────────────

// ReportPreview calls POST /reports/preview and returns the formatted preview text.
func (c *HTTPTriggerClient) ReportPreview(date string) (string, error) {
	return c.postText("/reports/preview", map[string]string{"date": date})
}

// ReportSend calls POST /reports/send to email the report to the given address.
func (c *HTTPTriggerClient) ReportSend(email, date string) (string, error) {
	return c.postText("/reports/send", map[string]string{"email": email, "date": date})
}

// ReportSave calls POST /reports/save and returns the saved-file message.
func (c *HTTPTriggerClient) ReportSave(date string) (string, error) {
	return c.postText("/reports/save", map[string]string{"date": date})
}

// ReportEOD calls POST /reports/eod (used by the daemon scheduler).
func (c *HTTPTriggerClient) ReportEOD(email, date string) (string, error) {
	return c.postText("/reports/eod", map[string]string{"email": email, "date": date})
}

// ── Learning methods ──────────────────────────────────────────────────────────

// LearningStatusResponse mirrors the /learning/status JSON payload.
type LearningStatusResponse struct {
	Enabled      bool   `json:"enabled"`
	ConsentGiven bool   `json:"consent_given"`
	SampleCount  int    `json:"sample_count"`
	LastUpdated  string `json:"last_updated"`
	Success      bool   `json:"success"`
}

// LearningStatus calls GET /learning/status.
func (c *HTTPTriggerClient) LearningStatus() (*LearningStatusResponse, error) {
	var r LearningStatusResponse
	if err := c.getWithResult("/learning/status", &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// LearningEnable calls POST /learning/enable.
func (c *HTTPTriggerClient) LearningEnable(days int) (string, error) {
	return c.postText("/learning/enable", map[string]any{"days": days})
}

// LearningSync calls POST /learning/sync.
func (c *HTTPTriggerClient) LearningSync(full bool) (string, error) {
	return c.postText("/learning/sync", map[string]any{"full": full})
}

// LearningReset calls POST /learning/reset.
func (c *HTTPTriggerClient) LearningReset() (string, error) {
	return c.postText("/learning/reset", map[string]any{})
}

// LearningSetupCron calls POST /learning/cron/setup.
func (c *HTTPTriggerClient) LearningSetupCron() (string, error) {
	return c.postText("/learning/cron/setup", map[string]any{})
}

// LearningRemoveCron calls DELETE /learning/cron.
func (c *HTTPTriggerClient) LearningRemoveCron() (string, error) {
	var r textOutput
	if err := c.deleteWithResult("/learning/cron", &r); err != nil {
		return "", err
	}
	if !r.Success {
		return r.Output, fmt.Errorf("server reported failure: %s", r.Output)
	}
	return r.Output, nil
}

// LearningCronStatus calls GET /learning/cron/status.
func (c *HTTPTriggerClient) LearningCronStatus() (string, error) {
	return c.getText("/learning/cron/status")
}

// LearningProfile calls GET /learning/profile.
func (c *HTTPTriggerClient) LearningProfile() (string, error) {
	return c.getText("/learning/profile")
}

// LearningTestResponse calls POST /learning/test-response.
func (c *HTTPTriggerClient) LearningTestResponse(text string) (string, error) {
	return c.postText("/learning/test-response", map[string]string{"text": text})
}

// LearningRevoke calls POST /learning/revoke.
func (c *HTTPTriggerClient) LearningRevoke() (string, error) {
	return c.postText("/learning/revoke", map[string]any{})
}

// ── Auth methods ──────────────────────────────────────────────────────────────

// AuthSessionResponse mirrors the session payload returned by auth endpoints.
type AuthSessionResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	LoggedIn         bool   `json:"logged_in"`
	Email            string `json:"email"`
	DisplayName      string `json:"display_name"`
	Tier             string `json:"tier"`
	Mode             string `json:"mode"`
	TelemetryEnabled bool   `json:"telemetry_enabled"`
	TokenExpiresAt   string `json:"token_expires_at"`
}

// AuthRequestMagicLink calls POST /auth/request-magic-link.
func (c *HTTPTriggerClient) AuthRequestMagicLink(email string) (string, error) {
	var r struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := c.postWithResult("/auth/request-magic-link", map[string]string{"email": email}, &r); err != nil {
		return "", err
	}
	if !r.Success {
		return r.Message, fmt.Errorf("%s", r.Message)
	}
	return r.Message, nil
}

// AuthVerifyMagicLink calls POST /auth/verify-magic-link.
func (c *HTTPTriggerClient) AuthVerifyMagicLink(email, code string) (*AuthSessionResponse, error) {
	var r AuthSessionResponse
	if err := c.postWithResult("/auth/verify-magic-link",
		map[string]string{"email": email, "code": code}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// AuthLogout calls POST /auth/logout.
func (c *HTTPTriggerClient) AuthLogout() (string, error) {
	var r struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := c.postWithResult("/auth/logout", map[string]any{}, &r); err != nil {
		return "", err
	}
	return r.Message, nil
}

// AuthWhoami calls GET /auth/whoami.
func (c *HTTPTriggerClient) AuthWhoami() (*AuthSessionResponse, error) {
	var r AuthSessionResponse
	if err := c.getWithResult("/auth/whoami", &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// AuthTelemetry calls POST /auth/telemetry with action "on"|"off"|"status".
func (c *HTTPTriggerClient) AuthTelemetry(action string) (string, error) {
	var r struct {
		Success          bool   `json:"success"`
		Message          string `json:"message"`
		TelemetryEnabled bool   `json:"telemetry_enabled"`
	}
	if err := c.postWithResult("/auth/telemetry", map[string]string{"action": action}, &r); err != nil {
		return "", err
	}
	return r.Message, nil
}

// ── License methods ───────────────────────────────────────────────────────────

// LicenseCheckResponse mirrors the /license/check response.
type LicenseCheckResponse struct {
	Accepted bool   `json:"accepted"`
	Success  bool   `json:"success"`
}

// LicenseIsAccepted calls GET /license/check and returns whether terms are accepted.
// Fails open (returns true) when the server is unreachable.
func (c *HTTPTriggerClient) LicenseIsAccepted() (bool, error) {
	if c.serverURL == "" {
		return true, nil // offline-safe: don't block
	}
	var r LicenseCheckResponse
	if err := c.getWithResult("/license/check", &r); err != nil {
		return true, nil // offline-safe: don't block on server error
	}
	return r.Accepted, nil
}

// LicenseStatus calls GET /license/status and returns formatted text.
func (c *HTTPTriggerClient) LicenseStatus() (string, error) {
	return c.getText("/license/status")
}

// LicenseTerms calls GET /license/terms and returns the T&C text.
func (c *HTTPTriggerClient) LicenseTerms() (string, error) {
	return c.getText("/license/terms")
}

// LicenseAccept calls POST /license/accept.
func (c *HTTPTriggerClient) LicenseAccept() (string, error) {
	var r struct {
		Accepted bool   `json:"accepted"`
		Success  bool   `json:"success"`
		Message  string `json:"message"`
	}
	if err := c.postWithResult("/license/accept", map[string]any{}, &r); err != nil {
		return "", err
	}
	return r.Message, nil
}
