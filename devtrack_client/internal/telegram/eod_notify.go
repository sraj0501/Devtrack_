package telegram

// eod_notify.go — Telegram delivery for EOD reports (TASK-078).
//
// Adds SendEODReport to the Bot struct so the queue executor can push an EOD
// report narrative to all configured Telegram chat IDs with Approve / Reject
// inline keyboard buttons (same callback_data pattern as TASK-065 queue actions).
//
// Delivery is gated on EOD_TELEGRAM_ENABLED=true. The function is a no-op when
// the bot is nil or the API connection has not been established yet.

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const eodNarrativeMaxLen = 4000

// SendEODReport delivers an EOD report narrative to all Telegram notify chat IDs
// with Approve and Reject inline keyboard buttons.
//
// Parameters:
//   - narrative: the AI-generated EOD narrative text (truncated to 4000 chars if longer)
//   - date:      the report date string, e.g. "2026-06-17"
//   - actionID:  the pending_actions row ID — used for approve/reject callback_data
//
// Message format:
//
//	[DevTrack] EOD Report — {date}
//	{narrative (truncated to 4000 chars, "…" appended if cut)}
//	[Approve] [Reject]
//
// Uses the EXACT same inline keyboard pattern as TASK-065 (callback_data
// "approve:<id>" / "reject:<id>") so existing callback handlers route these
// buttons correctly without any new code in queue_notify.go.
func (b *Bot) SendEODReport(narrative, date string, actionID int64) error {
	if b == nil || b.api == nil {
		return nil
	}

	text := formatEODReportMessage(narrative, date)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("approve:%d", actionID)),
			tgbotapi.NewInlineKeyboardButtonData("Reject", fmt.Sprintf("reject:%d", actionID)),
		),
	)

	var lastErr error
	for _, chatID := range b.notifyIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("telegram eod: failed to deliver EOD report to chat %d: %v", chatID, err)
			lastErr = err
		}
	}
	return lastErr
}

// formatEODReportMessage renders the Telegram message text for an EOD report.
// Narrative is truncated to eodNarrativeMaxLen (4000) characters to respect
// Telegram's 4096-character message limit; the header takes the remaining space.
func formatEODReportMessage(narrative, date string) string {
	truncated := narrative
	if len(truncated) > eodNarrativeMaxLen {
		truncated = truncated[:eodNarrativeMaxLen] + "…"
	}
	return fmt.Sprintf("*[DevTrack] EOD Report — %s*\n\n%s",
		escapeMarkdown(date),
		escapeMarkdown(truncated),
	)
}
