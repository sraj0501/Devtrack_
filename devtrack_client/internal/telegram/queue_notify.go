package telegram

// queue_notify.go — Telegram queue channel parity for TASK-065.
//
// Implements:
//   1. NotifyPendingAction — push message with Approve/Reject/Edit inline keyboard
//      when a new low-confidence action enters the pending queue.
//   2. handleCallbackQuery — processes inline button taps (approve/reject/edit).
//   3. /queue command — summary + list of pending actions with Approve/Reject buttons.
//   4. handlePotentialEditReply — captures text replies to pending "edit" prompts.
//
// No new external dependencies; uses the existing go-telegram-bot-api package.

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// QueueHooks wires the Telegram bot to the pending-actions queue without
// creating an import cycle. The daemon populates these before calling New.
type QueueHooks struct {
	// Database is the open SQLite handle used to read and update pending_actions rows.
	Database *db.Database
	// TriggerClient is used to call POST /queue/execute after an approval.
	TriggerClient *trigger.HTTPTriggerClient
}

// NotifyPendingAction sends a proactive Telegram message with Approve / Reject /
// Edit inline keyboard buttons to all notify chat IDs.
// Called by the QueueExecutor's NotifyFn when a new low-confidence action is seen.
// Safe to call concurrently.
func (b *Bot) NotifyPendingAction(action db.PendingAction) {
	if b.api == nil {
		return
	}

	text := formatPendingActionMessage(action)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("approve:%d", action.ID)),
			tgbotapi.NewInlineKeyboardButtonData("Reject", fmt.Sprintf("reject:%d", action.ID)),
			tgbotapi.NewInlineKeyboardButtonData("Edit", fmt.Sprintf("edit:%d", action.ID)),
		),
	)

	for _, chatID := range b.notifyIDs {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("telegram queue: failed to send pending action notification to chat %d: %v", chatID, err)
		}
	}
}

// handleCallbackQuery processes inline keyboard button taps.
// callback_data format: "<action>:<id>" e.g. "approve:42", "reject:42", "edit:42".
func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	if b.api == nil {
		return
	}

	// Always acknowledge the callback to remove the loading spinner in Telegram.
	ack := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(ack); err != nil {
		log.Printf("telegram queue: failed to ack callback %s: %v", query.ID, err)
	}

	if query.Message == nil {
		return
	}

	chatID := query.Message.Chat.ID
	if !b.isAuthorized(chatID) {
		log.Printf("telegram queue: unauthorised callback from chat %d", chatID)
		return
	}

	parts := strings.SplitN(query.Data, ":", 2)
	if len(parts) != 2 {
		log.Printf("telegram queue: malformed callback_data %q", query.Data)
		return
	}
	action := parts[0]
	actionID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		log.Printf("telegram queue: invalid action id in callback %q: %v", query.Data, err)
		return
	}

	switch action {
	case "approve":
		b.handleApproveCallback(chatID, query.Message.MessageID, actionID)
	case "reject":
		b.handleRejectCallback(chatID, query.Message.MessageID, actionID)
	case "edit":
		b.handleEditCallback(chatID, query.Message.MessageID, actionID)
	default:
		log.Printf("telegram queue: unknown callback action %q", action)
	}
}

// handleApproveCallback approves a pending action and dispatches it immediately.
func (b *Bot) handleApproveCallback(chatID int64, messageID int, actionID int64) {
	if b.hooks.Queue.Database == nil {
		b.editMessage(chatID, messageID, "Queue not available (database not wired).")
		return
	}

	if err := b.hooks.Queue.Database.UpdatePendingActionStatus(actionID, "approved", "telegram"); err != nil {
		log.Printf("telegram queue: approve %d: update status: %v", actionID, err)
		b.editMessage(chatID, messageID, fmt.Sprintf("Failed to approve action %d: %s", actionID, err.Error()))
		return
	}

	// Fire the action via the Python queue endpoint.
	if b.hooks.Queue.TriggerClient != nil {
		resp, err := b.hooks.Queue.TriggerClient.ExecuteQueueAction(actionID)
		if err != nil {
			log.Printf("telegram queue: approve %d: execute failed: %v", actionID, err)
			b.editMessage(chatID, messageID, fmt.Sprintf("Approved locally but dispatch failed: %s\nThe queue executor will retry.", err.Error()))
			return
		}
		if resp.Status == "failed" {
			b.editMessage(chatID, messageID, fmt.Sprintf("Approved but server reported failure: %s", resp.Error))
			return
		}
	}

	b.editMessage(chatID, messageID, fmt.Sprintf("Approved and dispatched. (action %d)", actionID))
}

// handleRejectCallback rejects a pending action. The action is never dispatched.
func (b *Bot) handleRejectCallback(chatID int64, messageID int, actionID int64) {
	if b.hooks.Queue.Database == nil {
		b.editMessage(chatID, messageID, "Queue not available (database not wired).")
		return
	}

	if err := b.hooks.Queue.Database.UpdatePendingActionStatus(actionID, "rejected", "telegram"); err != nil {
		log.Printf("telegram queue: reject %d: update status: %v", actionID, err)
		b.editMessage(chatID, messageID, fmt.Sprintf("Failed to reject action %d: %s", actionID, err.Error()))
		return
	}

	b.editMessage(chatID, messageID, fmt.Sprintf("Rejected. Action %d will not be dispatched.", actionID))
}

// handleEditCallback prompts the user to reply with new content for the action.
// The original message stays in place; we send a separate reply prompt.
func (b *Bot) handleEditCallback(chatID int64, messageID int, actionID int64) {
	// Record that this chat is waiting for an edit reply for actionID.
	b.pendingEditsMu.Lock()
	b.pendingEdits[chatID] = pendingEdit{ActionID: actionID, PromptMsgID: messageID}
	b.pendingEditsMu.Unlock()

	prompt := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"Reply to this message with your edited content for action %d.\n\n"+
			"Your reply will replace the action payload and approve it immediately.",
		actionID,
	))
	prompt.ParseMode = "Markdown"
	if _, err := b.api.Send(prompt); err != nil {
		log.Printf("telegram queue: edit prompt for action %d: %v", actionID, err)
	}
}

// handlePotentialEditReply checks whether a plain text message is a pending edit
// reply. If so, it updates the payload and approves the action.
// Returns true if the message was consumed as an edit reply, false otherwise.
func (b *Bot) handlePotentialEditReply(msg *tgbotapi.Message) bool {
	if msg.Text == "" || msg.IsCommand() {
		return false
	}

	b.pendingEditsMu.Lock()
	edit, waiting := b.pendingEdits[msg.Chat.ID]
	if waiting {
		delete(b.pendingEdits, msg.Chat.ID)
	}
	b.pendingEditsMu.Unlock()

	if !waiting {
		return false
	}

	if b.hooks.Queue.Database == nil {
		b.reply(msg, "Queue not available (database not wired).")
		return true
	}

	newPayload := msg.Text

	if err := b.hooks.Queue.Database.UpdatePendingActionPayload(edit.ActionID, newPayload); err != nil {
		log.Printf("telegram queue: edit reply %d: update payload: %v", edit.ActionID, err)
		b.reply(msg, fmt.Sprintf("Failed to update action %d payload: %s", edit.ActionID, err.Error()))
		return true
	}

	if err := b.hooks.Queue.Database.UpdatePendingActionStatus(edit.ActionID, "approved", "telegram"); err != nil {
		log.Printf("telegram queue: edit reply %d: approve: %v", edit.ActionID, err)
		b.reply(msg, fmt.Sprintf("Payload updated but failed to approve action %d: %s", edit.ActionID, err.Error()))
		return true
	}

	// Dispatch the edited action.
	if b.hooks.Queue.TriggerClient != nil {
		resp, err := b.hooks.Queue.TriggerClient.ExecuteQueueAction(edit.ActionID)
		if err != nil {
			b.reply(msg, fmt.Sprintf("Edited and approved locally, but dispatch failed: %s\nThe queue executor will retry.", err.Error()))
			b.editMessage(msg.Chat.ID, edit.PromptMsgID, fmt.Sprintf("Edited and approved. (action %d — dispatch pending)", edit.ActionID))
			return true
		}
		if resp.Status == "failed" {
			b.reply(msg, fmt.Sprintf("Edited but server reported failure: %s", resp.Error))
			return true
		}
	}

	b.editMessage(msg.Chat.ID, edit.PromptMsgID, fmt.Sprintf("Edited and dispatched. (action %d)", edit.ActionID))
	b.reply(msg, fmt.Sprintf("Action %d updated and dispatched.", edit.ActionID))
	return true
}

// handleQueueCommand implements the /queue bot command.
// Responds with: counts of pending/posted-today/rejected-today, then a list of
// each pending action with Approve/Reject inline buttons.
func (b *Bot) handleQueueCommand(msg *tgbotapi.Message) {
	if b.hooks.Queue.Database == nil {
		b.reply(msg, "Queue not available (database not wired).")
		return
	}

	pending, posted, rejected, err := b.hooks.Queue.Database.CountPendingActionsRecent()
	if err != nil {
		b.reply(msg, "Failed to query queue: "+err.Error())
		return
	}

	header := fmt.Sprintf("*DevTrack Queue*\nPending: %d | Posted today: %d | Rejected today: %d",
		pending, posted, rejected)

	if pending == 0 {
		b.reply(msg, header+"\n\nNo pending actions.")
		return
	}

	// Send the summary header first.
	b.reply(msg, header)

	// Then send each pending action as a separate message with Approve/Reject buttons.
	actions, err := b.hooks.Queue.Database.ListPendingActions("pending")
	if err != nil {
		b.reply(msg, "Failed to list pending actions: "+err.Error())
		return
	}

	for _, action := range actions {
		text := formatPendingActionMessage(action)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("approve:%d", action.ID)),
				tgbotapi.NewInlineKeyboardButtonData("Reject", fmt.Sprintf("reject:%d", action.ID)),
				tgbotapi.NewInlineKeyboardButtonData("Edit", fmt.Sprintf("edit:%d", action.ID)),
			),
		)
		out := tgbotapi.NewMessage(msg.Chat.ID, text)
		out.ParseMode = "Markdown"
		out.ReplyMarkup = keyboard
		if _, err := b.api.Send(out); err != nil {
			log.Printf("telegram queue: /queue list send error: %v", err)
		}
	}
}

// editMessage replaces the text of an existing Telegram message (used to update
// after approve/reject/edit so the original notification shows the outcome).
func (b *Bot) editMessage(chatID int64, messageID int, text string) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	if _, err := b.api.Send(edit); err != nil {
		log.Printf("telegram queue: edit message %d in chat %d: %v", messageID, chatID, err)
	}
}

// formatPendingActionMessage renders the notification text for a pending action.
func formatPendingActionMessage(action db.PendingAction) string {
	// Extract a short content preview from payload (best-effort JSON string value).
	content := payloadPreview(action.Payload, 80)
	confidence := int(action.Confidence * 100)
	window := confidenceWindowLabel(action.Confidence)
	remaining := remainingLabel(action.ExpiresAt)

	return fmt.Sprintf(
		"*[DevTrack] New pending action*\n"+
			"Type:        `%s`\n"+
			"Target:      `%s`\n"+
			"Platform:    `%s`\n"+
			"Content:     \"%s\"\n"+
			"Confidence:  %d%% (%s window)\n"+
			"Expires:     %s",
		escapeMarkdown(action.ActionType),
		escapeMarkdown(action.Target),
		escapeMarkdown(action.Platform),
		escapeMarkdown(content),
		confidence,
		window,
		remaining,
	)
}

// payloadPreview extracts a short readable string from a JSON payload.
// It looks for a top-level "text", "content", "comment", or "body" string field;
// falls back to the raw payload truncated to maxLen characters.
func payloadPreview(payload string, maxLen int) string {
	// Fast path: try common field names in order of likelihood.
	for _, key := range []string{"text", "content", "comment", "body", "description"} {
		needle := `"` + key + `":`
		idx := strings.Index(payload, needle)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(payload[idx+len(needle):])
		if len(rest) > 0 && rest[0] == '"' {
			// Find the closing quote (ignoring escaped quotes).
			end := 1
			for end < len(rest) {
				if rest[end] == '"' && rest[end-1] != '\\' {
					break
				}
				end++
			}
			if end < len(rest) {
				value := rest[1:end]
				if len(value) > maxLen {
					return value[:maxLen] + "..."
				}
				return value
			}
		}
	}
	// Fallback: return raw payload truncated.
	if len(payload) > maxLen {
		return payload[:maxLen] + "..."
	}
	return payload
}

// confidenceWindowLabel returns a human-readable approval window duration for
// the given confidence score, matching the ConfidenceTimeout logic in db.
func confidenceWindowLabel(confidence float64) string {
	if confidence > 0.90 {
		return "2m"
	}
	if confidence >= 0.70 {
		return "5m"
	}
	return "15m"
}

// remainingLabel returns "in Xm Ys", "in Xs", or "expired" for an ExpiresAt time.
func remainingLabel(expiresAt time.Time) string {
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return "expired"
	}
	remaining = remaining.Round(time.Second)
	mins := int(remaining.Minutes())
	secs := int(remaining.Seconds()) % 60
	if mins > 0 {
		return fmt.Sprintf("in %dm %ds", mins, secs)
	}
	return fmt.Sprintf("in %ds", secs)
}
