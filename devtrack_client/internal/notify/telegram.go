package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// Telegram delivers notifications to one or more Telegram chat IDs via the Bot API.
type Telegram struct {
	token    string
	chatIDs  []string
	http     *http.Client
}

// NewTelegramFromConfig returns a Telegram notifier configured from env vars,
// or nil when TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is not set.
func NewTelegramFromConfig() *Telegram {
	token := cfg.GetTelegramBotToken()
	ids := cfg.GetTelegramChatIDs()
	if token == "" || len(ids) == 0 {
		return nil
	}
	return &Telegram{
		token:   token,
		chatIDs: ids,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *Telegram) Send(title, body, url string) error {
	text := fmt.Sprintf("*%s*\n%s", escapeMarkdown(title), escapeMarkdown(body))
	if url != "" {
		text += "\n" + url
	}

	payload := map[string]any{
		"parse_mode": "Markdown",
		"text":       text,
	}

	var lastErr error
	for _, chatID := range t.chatIDs {
		payload["chat_id"] = chatID
		if err := t.sendMessage(payload); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (t *Telegram) sendMessage(payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)
	resp, err := t.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram: HTTP %d", resp.StatusCode)
	}
	return nil
}

// escapeMarkdown escapes Telegram Markdown v1 special chars.
func escapeMarkdown(s string) string {
	// Only escape the chars that break Markdown v1 formatting
	replacer := []struct{ from, to string }{
		{"_", "\\_"},
		{"*", "\\*"},
		{"`", "\\`"},
		{"[", "\\["},
	}
	for _, r := range replacer {
		for i := 0; i < len(s); i++ {
			if s[i] == r.from[0] {
				s = s[:i] + r.to + s[i+1:]
				i += len(r.to) - 1
			}
		}
	}
	return s
}
