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
	database          *db.Database
	notifier          notify.Notifier
	userID            string
	interval          time.Duration
	github            *githubAlerter
	azure             *azureAlerter
	reviewCommentHook func([]ReviewCommentEvent) // called after each cycle with new review events
	mergedPRHook      func([]MergedPREvent)      // called after each cycle with newly merged PRs (TASK-126)
	cancel            context.CancelFunc
}

// NewPoller creates a Poller that writes to database and delivers via notifier.
// reviewCommentHook is called with any new ReviewCommentEvents detected during
// a poll cycle. Pass nil to skip review comment polling. The hook is called
// synchronously inside the poll goroutine; keep it non-blocking.
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

// SetReviewCommentHook registers a callback that is invoked with any new
// ReviewCommentEvents detected during each poll cycle.
// Must be called before Start(); not goroutine-safe after Start() returns.
func (p *Poller) SetReviewCommentHook(hook func([]ReviewCommentEvent)) {
	p.reviewCommentHook = hook
}

// SetMergedPRHook registers a callback that is invoked with any PRs authored
// by the developer that merged into their repo's default branch since the last
// poll (TASK-126: "merged to main → Done").
// Must be called before Start(); not goroutine-safe after Start() returns.
func (p *Poller) SetMergedPRHook(hook func([]MergedPREvent)) {
	p.mergedPRHook = hook
}

// Start launches the poll loop in a background goroutine.
func (p *Poller) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	go p.run(ctx)
	log.Printf("Alert poller started (interval: %s)", p.interval)
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
	// Standard notification polling.
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

	// Merged-PR polling (TASK-126) — only when a hook is registered.
	// GitHub only for now; Azure/GitLab merged-PR detection not yet implemented.
	if p.mergedPRHook != nil && cfg.IsAlertGitHubEnabled() {
		if evs := p.github.collectMergedPRs(p.database, p.userID); len(evs) > 0 {
			p.mergedPRHook(evs)
		}
	}

	// Review comment polling — only when a hook is registered.
	if p.reviewCommentHook == nil {
		return
	}

	var reviewEvents []ReviewCommentEvent
	if cfg.IsAlertGitHubEnabled() {
		evs := p.github.collectReviewComments(p.database)
		reviewEvents = append(reviewEvents, evs...)
	}
	// Azure DevOps: TODO TASK-093 — review comment polling not yet implemented.
	if cfg.IsAlertAzureEnabled() {
		log.Printf("review polling: azure not yet implemented")
	}
	// GitLab: TODO TASK-093 — review comment polling not yet implemented.
	// log.Printf("review polling: gitlab not yet implemented")

	if len(reviewEvents) > 0 {
		p.reviewCommentHook(reviewEvents)
	}
}
