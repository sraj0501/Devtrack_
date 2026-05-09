package main

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
)

// HealthMonitor periodically checks all services and records results
type HealthMonitor struct {
	db            *Database
	checkInterval time.Duration
	stopCh        chan struct{}
	running       bool
	mu            sync.Mutex
	dbMu          sync.Mutex // serializes SQLite writes from concurrent check goroutines

	// Process PIDs to monitor
	webhookPID  int
	telegramPID int

	// Auto-restart callbacks
	restartWebhook  func() error
	restartTelegram func() error

	// Restart tracking
	restartCounts   map[string][]time.Time // service -> timestamps of restarts
	maxRestartsHour int
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(db *Database) *HealthMonitor {
	return &HealthMonitor{
		db:              db,
		checkInterval:   time.Duration(GetHealthCheckIntervalSecs()) * time.Second,
		stopCh:          make(chan struct{}),
		restartCounts:   make(map[string][]time.Time),
		maxRestartsHour: GetHealthMaxRestartsPerHour(),
	}
}

// SetWebhookPID sets the webhook server PID to monitor
func (hm *HealthMonitor) SetWebhookPID(pid int) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.webhookPID = pid
}

// SetTelegramPID sets the Telegram bot PID to monitor
func (hm *HealthMonitor) SetTelegramPID(pid int) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.telegramPID = pid
}

// SetRestartCallbacks sets the functions to call when auto-restart is needed
func (hm *HealthMonitor) SetRestartCallbacks(_, restartWebhook func() error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.restartWebhook = restartWebhook
}

// SetTelegramRestartCallback sets the function to call when the Telegram bot needs restart
func (hm *HealthMonitor) SetTelegramRestartCallback(fn func() error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.restartTelegram = fn
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
		hm.checkMongoDB,
		hm.checkRedis,
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
	snap := HealthSnapshot{
		Service:   "python_http",
		CheckedAt: time.Now(),
	}

	client := NewHTTPTriggerClient()
	if client.Ping() {
		snap.Status = "up"
		snap.LatencyMs = int(time.Since(start).Milliseconds())
		snap.Details = fmt.Sprintf(`{"url":%q,"latency_ms":%d}`, GetServerURL(), snap.LatencyMs)
	} else {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"url":%q,"error":"no response"}`, GetServerURL())
	}

	hm.recordSnapshot(snap)
}

// normalizeOllamaHost converts an OLLAMA_HOST value into a proper base URL
// suitable for outbound HTTP connections.
//
// OLLAMA_HOST is Ollama's bind-address env var, so users often set it to
// "0.0.0.0", "0.0.0.0:11434", or just "11434". All of those are valid for
// Ollama itself but not for an outbound dial — this function canonicalises
// them into "http://localhost:<port>".
func normalizeOllamaHost(raw string) string {
	raw = strings.TrimRight(raw, "/")
	if raw == "" {
		return "http://localhost:11434"
	}

	// Add scheme if missing so url.Parse works correctly.
	withScheme := raw
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		withScheme = "http://" + raw
	}

	u, err := url.Parse(withScheme)
	if err != nil {
		// Unparseable — return a safe default.
		return "http://localhost:11434"
	}

	host := u.Hostname()
	port := u.Port()

	// 0.0.0.0 is a bind-all address; connect to localhost instead.
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	// No port in the URL — use Ollama's default.
	if port == "" {
		port = "11434"
	}

	return fmt.Sprintf("%s://%s", u.Scheme, net.JoinHostPort(host, port))
}

// checkPythonBridge checks if the Python bridge process is alive
// checkOllama checks if Ollama is reachable
func (hm *HealthMonitor) checkOllama() {
	snap := HealthSnapshot{
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
	snap := HealthSnapshot{
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
	snap := HealthSnapshot{
		Service:   "webhook_server",
		CheckedAt: time.Now(),
	}

	if !IsWebhookEnabled() {
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
	} else if isProcessAlive(pid) {
		snap.Status = "up"
		port := GetWebhookPort()
		snap.Details = fmt.Sprintf(`{"pid":%d,"port":%d}`, pid, port)
	} else {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"pid":%d,"error":"process not running"}`, pid)
		if GetHealthAutoRestartWebhook() {
			hm.tryRestart("webhook_server")
		}
	}

	hm.recordSnapshot(snap)
}

// checkTelegramBot checks if the Telegram bot process is alive
func (hm *HealthMonitor) checkTelegramBot() {
	snap := HealthSnapshot{
		Service:   "telegram_bot",
		CheckedAt: time.Now(),
	}

	if !IsTelegramEnabled() {
		snap.Status = "unconfigured"
		snap.Details = `{"enabled":false}`
		hm.recordSnapshot(snap)
		return
	}

	hm.mu.Lock()
	pid := hm.telegramPID
	hm.mu.Unlock()

	if pid == 0 {
		snap.Status = "down"
		snap.Details = `{"error":"not started"}`
	} else if isProcessAlive(pid) {
		snap.Status = "up"
		snap.Details = fmt.Sprintf(`{"pid":%d}`, pid)
	} else {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"pid":%d,"error":"process not running"}`, pid)
		if GetHealthAutoRestartTelegram() {
			hm.tryRestart("telegram_bot")
		}
	}

	hm.recordSnapshot(snap)
}

// checkRedis checks if Redis is reachable
func (hm *HealthMonitor) checkRedis() {
	snap := HealthSnapshot{
		Service:   "redis",
		CheckedAt: time.Now(),
	}

	if os.Getenv("REDIS_URL") == "" {
		snap.Status = "unconfigured"
		snap.Details = `{"error":"REDIS_URL not set"}`
		hm.recordSnapshot(snap)
		return
	}

	redisHost, redisPort, _ := resolveRedisConfig()
	host := net.JoinHostPort(redisHost, redisPort)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	latency := time.Since(start)

	if err != nil {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"url":%q,"error":%q}`, host, err.Error())
	} else {
		conn.Close()
		snap.Status = "up"
		snap.LatencyMs = int(latency.Milliseconds())
		snap.Details = fmt.Sprintf(`{"url":%q,"latency_ms":%d}`, host, snap.LatencyMs)
	}

	hm.recordSnapshot(snap)
}

// checkSQLite checks if the local SQLite database file is accessible
func (hm *HealthMonitor) checkSQLite() {
	snap := HealthSnapshot{
		Service:   "sqlite",
		CheckedAt: time.Now(),
	}

	dbPath := GetDatabasePath()
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

// checkMongoDB checks if MongoDB is reachable
func (hm *HealthMonitor) checkMongoDB() {
	snap := HealthSnapshot{
		Service:   "mongodb",
		CheckedAt: time.Now(),
	}

	if os.Getenv("MONGODB_URI") == "" {
		snap.Status = "unconfigured"
		snap.Details = `{"error":"MONGODB_URI not set"}`
		hm.recordSnapshot(snap)
		return
	}

	// Derive host:port from MONGODB_URI (may include a Docker-assigned port)
	mongoHost, mongoPort, _, _, _ := resolveMongoConfig()
	host := net.JoinHostPort(mongoHost, mongoPort)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	latency := time.Since(start)

	if err != nil {
		snap.Status = "down"
		snap.Details = fmt.Sprintf(`{"url":%q,"error":%q}`, host, err.Error())
	} else {
		conn.Close()
		snap.Status = "up"
		snap.LatencyMs = int(latency.Milliseconds())
		snap.Details = fmt.Sprintf(`{"url":%q,"latency_ms":%d}`, host, snap.LatencyMs)
	}

	hm.recordSnapshot(snap)
}

// tryRestart attempts to restart a service if within rate limits
func (hm *HealthMonitor) tryRestart(service string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	// Prune old restart timestamps
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
	case "telegram_bot":
		restartFn = hm.restartTelegram
	}

	if restartFn == nil {
		return
	}

	log.Printf("Health: auto-restarting %s...", service)
	hm.restartCounts[service] = append(hm.restartCounts[service], now)

	// Run restart in goroutine to avoid blocking health checks
	go func() {
		if err := restartFn(); err != nil {
			log.Printf("Health: failed to restart %s: %v", service, err)
		} else {
			log.Printf("Health: %s restarted successfully", service)
		}
	}()
}

// recordSnapshot writes a health snapshot to the database.
// dbMu serializes writes so concurrent check goroutines don't cause SQLITE_BUSY.
func (hm *HealthMonitor) recordSnapshot(snap HealthSnapshot) {
	if hm.db == nil {
		return
	}
	hm.dbMu.Lock()
	defer hm.dbMu.Unlock()
	if err := hm.db.InsertHealthSnapshot(snap); err != nil {
		log.Printf("Health: failed to record snapshot for %s: %v", snap.Service, err)
	}
}

// isProcessAlive checks if a process with the given PID is running
func isProcessAlive(pid int) bool {
	return checkProcessAlive(pid)
}
