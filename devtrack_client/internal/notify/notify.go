// Package notify provides notification delivery for ticket alerts.
// Supported channels: terminal (stdout), OS native, Telegram Bot API, Slack webhook.
package notify

import (
	"fmt"
	"log"
)

// Notifier delivers a single notification.
type Notifier interface {
	Send(title, body, url string) error
}

// Multi delivers to all registered notifiers, logging but not stopping on errors.
type Multi struct {
	notifiers []Notifier
}

// NewMulti builds a Multi notifier from the given list (nil entries are skipped).
func NewMulti(notifiers ...Notifier) *Multi {
	var active []Notifier
	for _, n := range notifiers {
		if n != nil {
			active = append(active, n)
		}
	}
	return &Multi{notifiers: active}
}

func (m *Multi) Send(title, body, url string) error {
	for _, n := range m.notifiers {
		if err := n.Send(title, body, url); err != nil {
			log.Printf("notify: %T: %v", n, err)
		}
	}
	return nil
}

// Terminal prints the notification to stdout.
type Terminal struct{}

func (Terminal) Send(title, body, url string) error {
	if url != "" {
		fmt.Printf("\n  [alert] %s\n  %s\n  %s\n", title, body, url)
	} else {
		fmt.Printf("\n  [alert] %s — %s\n", title, body)
	}
	return nil
}
