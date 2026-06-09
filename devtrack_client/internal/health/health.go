package health

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	idb "github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)


// HealthMonitor periodically checks all services and records results
type HealthMonitor struct {
	db            *idb.Database
	checkInterval time.Duration
	stopCh        chan struct{}
	running       bool
	mu            sync.Mutex
	dbMu          sync.Mutex // serializes SQLite writes from concurrent check goroutines

	// Process PIDs to monitor
	webhookPID int

	// Auto-restart callbacks
	restartWebhook func() error

	// Restart tracking
	restartCounts   map[string][]time.Time // service -> timestamps of restarts
	maxRestartsHour int

	// Ping function injected from main to avoid circular dependency
	PingServerFn func() bool
	GetServerURL func() string
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(db *idb.Database) *HealthMonitor {
	return &HealthMonitor{
		db:              db,
		checkInterval:   time.Duration(cfg.GetHealthCheckIntervalSecs()) * time.Second,
		stopCh:          make(chan struct{}),
		restartCounts:   make(map[string][]time.Time),
		maxRestartsHour: cfg.GetHealthMaxRestartsPerHour(),
	}
}

// SetWebhookPID sets the webhook server PID to monitor
func (hm *HealthMonitor) SetWebhookPID(pid int) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.webhookPID = pid
}

// SetRestartCallbacks sets the functions to call when auto-restart is needed
func (hm *HealthMonitor) SetRestartCallbacks(_, restartWebhook func() error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.restartWebhook = restartWebhook
}

// Start begins periodic health checking
func (hm *HealthMonitor) Start() {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = true
	hm.mu.Unlock()

	log.Printf("Health monitor started (interval: %s)", hm.checkInterval)

	go func() {
		// Initial check after short delay (all checks run concurrently so 2s is enough)
		time.Sleep(2 * time.Second)
		hm.RunAllChecks()

		ticker := time.NewTicker(hm.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hm.RunAllChecks()
			case <-hm.stopCh:
				log.Println("Health monitor stopped")
				return
			}
		}
	}()
}

// Stop stops the health monitor
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	if hm.running {
		close(hm.stopCh)
		hm.running = false
	}
}

// RunAllChecks runs all health checks concurrently and records results.
// Running in parallel prevents a slow/unreachable service from delaying
// the status display for all other services.
func (hm *HealthMonitor) RunAllChecks() {
	var wg sync.WaitGroup
	checks := []func(){
		hm.checkPythonHTTP,
		hm.checkOllama,
		hm.checkAzureDevOps,
		hm.checkWebhookServer,
		hm.checkTelegramBot,
		hm.checkSQLite,
	}
	for _, check := range checks {
		wg.Add(1)
		go func(fn func()) {
			defer wg.Done()
			fn()
		}(check)
	}
	wg.Wait()
}

// checkPythonHTTP verifies the Python HTTP server responds to /health.
func (hm *HealthMonitor) checkPythonHTTP() {
	start := time.Now()
	snap := idb.HealthSnapshot{
		Service:   "python_http",
		CheckedAt: time.Now(),
	}

	serverURL := ""
	if hm.GetServerURL != nil {
		serverURL = hm.GetServerURL()
	}

	if hm.PingServerFn != nil && hm.PingServerFn() {
		snap.Status = "up"
		snap.LatencyMs = int(time.Since(start).Milliseconds())
		snap.Details = fmt.Sprintf(`{"url":%q,"latency_ms":%d}`, serverURL, snap.LatencyMs)
	} else {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"url":%q,"error":"no response"}`, serverURL)
	}

	hm.recordSnapshot(snap)
}

// normalizeOllamaHost converts an OLLAMA_HOST value into a proper base URL
// suitable for outbound HTTP connections.
func normalizeOllamaHost(raw string) string {
	raw = strings.TrimRight(raw, "/")
	if raw == "" {
		return "http://localhost:11434"
	}

	withScheme := raw
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		withScheme = "http://" + raw
	}

	u, err := url.Parse(withScheme)
	if err != nil {
		return "http://localhost:11434"
	}

	host := u.Hostname()
	port := u.Port()

	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	if port == "" {
		port = "11434"
	}

	return fmt.Sprintf("%s://%s", u.Scheme, net.JoinHostPort(host, port))
}

// checkOllama checks if Ollama is reachable
func (hm *HealthMonitor) checkOllama() {
	snap := idb.HealthSnapshot{
		Service:   "ollama",
		CheckedAt: time.Now(),
	}

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		snap.Status = "unconfigured"
		snap.Details = `{"error":"OLLAMA_HOST not set"}`
		hm.recordSnapshot(snap)
		return
	}
	ollamaHost = normalizeOllamaHost(ollamaHost)
	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ollamaHost + "/api/tags")
	latency := time.Since(start)

	if err != nil {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"url":%q,"error":%q}`, ollamaHost, err.Error())
	} else {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			snap.Status = "up"
			snap.LatencyMs = int(latency.Milliseconds())
			snap.Details = fmt.Sprintf(`{"url":%q,"latency_ms":%d}`, ollamaHost, snap.LatencyMs)
		} else {
			snap.Status = "degraded"
			snap.Details = fmt.Sprintf(`{"url":%q,"status_code":%d}`, ollamaHost, resp.StatusCode)
		}
	}

	hm.recordSnapshot(snap)
}

// checkAzureDevOps checks if Azure DevOps is configured
func (hm *HealthMonitor) checkAzureDevOps() {
	snap := idb.HealthSnapshot{
		Service:   "azure_devops",
		CheckedAt: time.Now(),
	}

	pat := os.Getenv("AZURE_DEVOPS_PAT")
	org := os.Getenv("AZURE_ORGANIZATION")
	project := os.Getenv("AZURE_PROJECT")

	if pat == "" || org == "" || project == "" {
		snap.Status = "unconfigured"
		details := map[string]bool{
			"pat_set":     pat != "",
			"org_set":     org != "",
			"project_set": project != "",
		}
		detailsJSON, _ := json.Marshal(details)
		snap.Details = string(detailsJSON)
	} else {
		snap.Status = "up"
		snap.Details = fmt.Sprintf(`{"org":%q}`, org)
	}

	hm.recordSnapshot(snap)
}

// checkWebhookServer checks if the webhook server process is alive
func (hm *HealthMonitor) checkWebhookServer() {
	snap := idb.HealthSnapshot{
		Service:   "webhook_server",
		CheckedAt: time.Now(),
	}

	if !cfg.IsWebhookEnabled() {
		snap.Status = "unconfigured"
		snap.Details = `{"enabled":false}`
		hm.recordSnapshot(snap)
		return
	}

	hm.mu.Lock()
	pid := hm.webhookPID
	hm.mu.Unlock()

	if pid == 0 {
		snap.Status = "down"
		snap.Details = `{"error":"not started"}`
	} else if IsProcessAlive(pid) {
		snap.Status = "up"
		port := cfg.GetWebhookPort()
		snap.Details = fmt.Sprintf(`{"pid":%d,"port":%d}`, pid, port)
	} else {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"pid":%d,"error":"process not running"}`, pid)
		if cfg.GetHealthAutoRestartWebhook() {
			hm.tryRestart("webhook_server")
		}
	}

	hm.recordSnapshot(snap)
}

// checkTelegramBot checks if the Telegram notifier is properly configured.
// Phase 2: Telegram is now a native Go notifier (not a subprocess), so we
// check whether the required credentials are present rather than a PID.
func (hm *HealthMonitor) checkTelegramBot() {
	snap := idb.HealthSnapshot{
		Service:   "telegram_bot",
		CheckedAt: time.Now(),
	}

	if !cfg.IsTelegramEnabled() {
		snap.Status = "unconfigured"
		snap.Details = `{"enabled":false}`
		hm.recordSnapshot(snap)
		return
	}

	token := cfg.GetTelegramBotToken()
	ids := cfg.GetTelegramChatIDs()
	if token == "" || len(ids) == 0 {
		snap.Status = "down"
		snap.Details = `{"error":"TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID not set"}`
	} else {
		snap.Status = "up"
		snap.Details = fmt.Sprintf(`{"chat_ids":%d}`, len(ids))
	}

	hm.recordSnapshot(snap)
}


// checkSQLite checks if the local SQLite database file is accessible
func (hm *HealthMonitor) checkSQLite() {
	snap := idb.HealthSnapshot{
		Service:   "sqlite",
		CheckedAt: time.Now(),
	}

	dbPath := cfg.GetDatabasePath()
	info, err := os.Stat(dbPath)
	if err != nil {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"error":%q,"path":%q}`, err.Error(), dbPath)
	} else {
		snap.Status = "up"
		snap.Details = fmt.Sprintf(`{"size_kb":%d,"path":%q}`, info.Size()/1024, dbPath)
	}

	hm.recordSnapshot(snap)
}


// tryRestart attempts to restart a service if within rate limits
func (hm *HealthMonitor) tryRestart(service string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	recent := []time.Time{}
	for _, t := range hm.restartCounts[service] {
		if t.After(oneHourAgo) {
			recent = append(recent, t)
		}
	}
	hm.restartCounts[service] = recent

	if len(recent) >= hm.maxRestartsHour {
		log.Printf("Health: %s restart skipped — %d restarts in last hour (max %d)",
			service, len(recent), hm.maxRestartsHour)
		return
	}

	var restartFn func() error
	switch service {
	case "webhook_server":
		restartFn = hm.restartWebhook
	}

	if restartFn == nil {
		return
	}

	log.Printf("Health: auto-restarting %s...", service)
	hm.restartCounts[service] = append(hm.restartCounts[service], now)

	go func() {
		if err := restartFn(); err != nil {
			log.Printf("Health: failed to restart %s: %v", service, err)
		} else {
			log.Printf("Health: %s restarted successfully", service)
		}
	}()
}

// recordSnapshot writes a health snapshot to the database.
func (hm *HealthMonitor) recordSnapshot(snap idb.HealthSnapshot) {
	if hm.db == nil {
		return
	}
	hm.dbMu.Lock()
	defer hm.dbMu.Unlock()
	if err := hm.db.InsertHealthSnapshot(snap); err != nil {
		log.Printf("Health: failed to record snapshot for %s: %v", snap.Service, err)
	}
}

// IsProcessAlive checks if a process with the given PID is running.
// Must be set by the main package (platform-specific) before using HealthMonitor.
var IsProcessAlive func(pid int) bool

