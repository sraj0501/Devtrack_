package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

// Slack delivers notifications via an incoming webhook URL.
type Slack struct {
	webhookURL string
	http       *http.Client
}

// NewSlackFromConfig returns a Slack notifier configured from SLACK_WEBHOOK_URL,
// or nil when the var is not set.
func NewSlackFromConfig() *Slack {
	url := cfg.GetSlackWebhookURL()
	if url == "" {
		return nil
	}
	return &Slack{
		webhookURL: url,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Slack) Send(title, body, url string) error {
	text := fmt.Sprintf("*%s* — %s", title, body)
	if url != "" {
		text += "\n" + url
	}

	payload := map[string]string{"text": text}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := s.http.Post(s.webhookURL, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("slack: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack: HTTP %d", resp.StatusCode)
	}
	return nil
}
