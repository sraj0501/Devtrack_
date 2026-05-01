package contract

// Route paths — single source of truth for both server and CLI.
const (
	RouteHealth       = "/health"
	RouteStart        = "/start"
	RouteStop         = "/stop"
	RouteStatus       = "/status"
	RouteLogs         = "/logs"
	RoutePause        = "/pause"
	RouteResume       = "/resume"
	RouteForceTrigger = "/trigger/force"
	RouteVersion      = "/version"
)

// AuthHeader is checked on every request when DEVTRACK_API_TOKEN is set.
const AuthHeader = "X-DevTrack-Token"

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	Running    bool   `json:"running"`
	PID        int    `json:"pid,omitempty"`
	Uptime     string `json:"uptime,omitempty"`
	Monitoring string `json:"monitoring,omitempty"`
	Paused     bool   `json:"paused"`
}

// LogsResponse is returned by GET /logs.
type LogsResponse struct {
	Lines []string `json:"lines"`
}

// LogsRequest carries optional query params for GET /logs.
type LogsRequest struct {
	Tail int `json:"tail,omitempty"` // number of lines, default 50
}

// CommandResponse is the generic reply for POST commands (start/stop/pause/resume/trigger).
type CommandResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// VersionResponse is returned by GET /version.
type VersionResponse struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	BuildTime string `json:"build_time,omitempty"`
}

// ErrorResponse wraps API errors.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}
