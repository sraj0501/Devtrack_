package alerts

import (
	"context"
	"log"
	"time"

	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/notify"
)

// Poller polls GitHub and Azure for ticket alert notifications on a configurable
// interval, writes new ones to SQLite, and delivers them via the notifier.
type Poller struct {
	database *db.Database
	notifier notify.Notifier
	userID   string
	interval time.Duration
	github   *githubAlerter
	azure    *azureAlerter
	cancel   context.CancelFunc
}

// NewPoller creates a Poller that writes to database and delivers via notifier.
func NewPoller(database *db.Database, notifier notify.Notifier) *Poller {
	filter := FilterFromConfig()
	return &Poller{
		database: database,
		notifier: notifier,
		userID:   cfg.GetAlertUserID(),
		interval: time.Duration(cfg.GetAlertPollIntervalSecs()) * time.Second,
		github:   &githubAlerter{filter: filter},
		azure:    &azureAlerter{filter: filter},
	}
}

// Start launches the poll loop in a background goroutine.
func (p *Poller) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	go p.run(ctx)
	log.Printf("✓ Alert poller started (interval: %s)", p.interval)
}

// Stop cancels the poll loop.
func (p *Poller) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *Poller) run(ctx context.Context) {
	p.cycle()
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cycle()
		}
	}
}

func (p *Poller) cycle() {
	var candidates []db.NotificationRecord

	if cfg.IsAlertGitHubEnabled() {
		candidates = append(candidates, p.github.collect(p.database, p.userID)...)
	}
	if cfg.IsAlertAzureEnabled() {
		candidates = append(candidates, p.azure.collect(p.database, p.userID)...)
	}

	for _, r := range candidates {
		inserted, err := p.database.InsertNotificationNew(r)
		if err != nil {
			log.Printf("alerts: insert: %v", err)
			continue
		}
		if inserted && p.notifier != nil {
			label := r.EventType + " — " + r.Source
			_ = p.notifier.Send(r.Title, label, r.URL)
		}
	}
}
