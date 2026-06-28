package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendPRApproved sends a "[DevTrack] PR Approved" notification to all configured
// notify chat IDs. This is a plain-text status message — no inline keyboard is
// needed because the developer has no action to take.
//
// Safe to call on a nil receiver (Telegram disabled or bot token missing).
func (b *Bot) SendPRApproved(prTitle, prURL string) error {
	if b == nil || b.api == nil {
		return nil
	}
	text := fmt.Sprintf(
		"[DevTrack] PR Approved\n%s\n%s\n\nAll review comments resolved automatically.",
		prTitle, prURL,
	)
	var lastErr error
	for _, chatID := range b.notifyIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		if _, err := b.api.Send(msg); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// SendPREscalation sends a "[DevTrack] PR Needs You" escalation notification to
// all configured notify chat IDs. Called when the fix loop gives up on a comment
// that cannot be automatically resolved.
//
// No inline keyboard is attached — the developer must inspect the PR directly.
// Safe to call on a nil receiver (Telegram disabled or bot token missing).
func (b *Bot) SendPREscalation(prTitle, blockerReason, commentURL, prURL string) error {
	if b == nil || b.api == nil {
		return nil
	}
	text := fmt.Sprintf(
		"[DevTrack] PR Needs You\n%s\n%s\n\nBlocker: %s\nComment: %s\n\nDevTrack attempted fixes but could not resolve this comment.",
		prTitle, prURL, blockerReason, commentURL,
	)
	var lastErr error
	for _, chatID := range b.notifyIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		if _, err := b.api.Send(msg); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
