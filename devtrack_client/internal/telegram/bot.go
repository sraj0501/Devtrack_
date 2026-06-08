// Package telegram provides the interactive Telegram bot for DevTrack.
// It handles both inbound commands from the user and outbound push notifications
// to configured chat IDs, replacing the separate Python telegram bot process.
package telegram

import (
	"context"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// Hooks wires the bot to daemon operations without creating an import cycle.
// The daemon populates these before calling New; nil hooks are silently skipped.
type Hooks struct {
	StatusText    func() string
	LogLines      func(n int) string
	HealthText    func() string
	ForceTrigger  func() error
	Pause         func() error
	Resume        func() error
	Stop          func()
	Restart       func()
	ReloadConfig  func() error
	RecentCommits func(n int) string
}

// Bot is the interactive Telegram bot for DevTrack.
// It implements notify.Notifier so it can be passed to the alert poller.
type Bot struct {
	api        *tgbotapi.BotAPI
	allowedIDs map[int64]struct{}
	notifyIDs  []int64
	hooks      Hooks
	ctx        context.Context
	cancel     context.CancelFunc
}

// New creates a Bot from environment config.
// Returns nil when Telegram is disabled or TELEGRAM_BOT_TOKEN is absent.
func New(hooks Hooks) *Bot {
	if !config.IsTelegramEnabled() {
		return nil
	}
	token := config.GetTelegramBotToken()
	if token == "" {
		return nil
	}

	allowedIDs := make(map[int64]struct{})
	for _, s := range config.GetTelegramAllowedChatIDs() {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			allowedIDs[id] = struct{}{}
		}
	}

	var notifyIDs []int64
	for _, s := range config.GetTelegramChatIDs() {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			notifyIDs = append(notifyIDs, id)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Bot{
		allowedIDs: allowedIDs,
		notifyIDs:  notifyIDs,
		hooks:      hooks,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start connects to the Telegram API and begins polling for updates.
func (b *Bot) Start() error {
	api, err := tgbotapi.NewBotAPI(config.GetTelegramBotToken())
	if err != nil {
		return err
	}
	b.api = api
	log.Printf("✓ Telegram bot started (@%s)", api.Self.UserName)
	if len(b.allowedIDs) == 0 {
		log.Println("  Warning: TELEGRAM_ALLOWED_CHAT_IDS is empty — bot accepts commands from anyone")
	}
	go b.poll()
	return nil
}

// Stop shuts down the polling loop.
func (b *Bot) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.api != nil {
		b.api.StopReceivingUpdates()
	}
}

// Send implements notify.Notifier — delivers a push message to all notify chat IDs.
func (b *Bot) Send(title, body, url string) error {
	if b.api == nil {
		return nil
	}
	text := "*" + escapeMarkdown(title) + "*\n" + escapeMarkdown(body)
	if url != "" {
		text += "\n" + url
	}
	var lastErr error
	for _, chatID := range b.notifyIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		if _, err := b.api.Send(msg); err != nil {
			lastErr = err
			log.Printf("telegram: notify to %d: %v", chatID, err)
		}
	}
	return lastErr
}

func (b *Bot) isAuthorized(chatID int64) bool {
	if len(b.allowedIDs) == 0 {
		return true // dev mode
	}
	_, ok := b.allowedIDs[chatID]
	return ok
}

func (b *Bot) poll() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.api.GetUpdatesChan(u)
	for {
		select {
		case <-b.ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message == nil || !update.Message.IsCommand() {
				continue
			}
			go b.handleCommand(update.Message)
		}
	}
}

func (b *Bot) reply(msg *tgbotapi.Message, text string) {
	out := tgbotapi.NewMessage(msg.Chat.ID, text)
	out.ParseMode = "Markdown"
	if _, err := b.api.Send(out); err != nil {
		log.Printf("telegram: reply error: %v", err)
	}
}

func escapeMarkdown(s string) string {
	r := strings.NewReplacer("_", "\\_", "*", "\\*", "`", "\\`", "[", "\\[")
	return r.Replace(s)
}
