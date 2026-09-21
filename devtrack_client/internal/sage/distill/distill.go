// Package distill owns Sage model prompts and validates structured knowledge
// drafts. It is designed for daemon-owned background workers only; capture
// hooks must never call this package.
package distill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
)

var signatureComment = regexp.MustCompile(`(?i)<!--\s*sig:\s*[0-9a-f]{6,40}\s*-->`)

type Fact struct {
	Command string
	Failed  bool
}

type Draft struct {
	Title    string
	Section  string
	Commands []string
	What     string
	Why      string
	Example  string
	Notes    string
}

type Outcome struct {
	Draft      *Draft
	Skipped    bool
	SkipReason string
}

// RetryableError means infrastructure or model output failed, not that the
// captured command was judged unworthy. Durable queue owners should retry it.
type RetryableError struct {
	Cause error
}

func (e *RetryableError) Error() string { return e.Cause.Error() }
func (e *RetryableError) Unwrap() error { return e.Cause }

func IsRetryable(err error) bool {
	var target *RetryableError
	return errors.As(err, &target)
}

type Client interface {
	ChatJSONContext(context.Context, []llmclient.Message) (string, error)
}

type Distiller struct {
	Client  Client
	Timeout time.Duration
}

func (d Distiller) Distill(ctx context.Context, fact Fact) (Outcome, error) {
	if d.Client == nil {
		return Outcome{}, &RetryableError{Cause: errors.New("sage distiller has no model client")}
	}
	command := strings.TrimSpace(fact.Command)
	if command == "" {
		return Outcome{}, &RetryableError{Cause: errors.New("sage distillation fact has no command")}
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prompt := entryPrompt(command, fact.Failed)
	raw, err := d.Client.ChatJSONContext(requestCtx, []llmclient.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return Outcome{}, &RetryableError{Cause: fmt.Errorf("sage model request: %w", err)}
	}
	outcome, err := Parse(raw, command)
	if err != nil {
		return Outcome{}, &RetryableError{Cause: err}
	}
	return outcome, nil
}

const systemPrompt = `You write one reusable personal command-reference entry. Return one JSON object only. The object must contain worth_documenting, skip_reason, title, section, commands, what, why, example, and notes. Never emit a signature, filename, route, or Markdown. Replace secrets with <TOKEN> and personal absolute paths with <repo>/... or ~/....`

func entryPrompt(command string, failed bool) string {
	payload, _ := json.Marshal(struct {
		Command string `json:"command"`
		Failed  bool   `json:"failed"`
	}{Command: command, Failed: failed})
	return "Document this captured fact:\n" + string(payload)
}

type response struct {
	WorthDocumenting *bool    `json:"worth_documenting"`
	SkipReason       string   `json:"skip_reason"`
	Title            string   `json:"title"`
	Heading          string   `json:"heading"`
	Section          string   `json:"section"`
	Commands         []string `json:"commands"`
	What             string   `json:"what"`
	Why              string   `json:"why"`
	Example          string   `json:"example"`
	Notes            string   `json:"notes"`
}

// Parse distinguishes a deliberate skip from malformed model output. Only a
// deliberate skip may leave the retry queue without producing a draft.
func Parse(raw, capturedCommand string) (Outcome, error) {
	body, err := firstJSONObject(raw)
	if err != nil {
		return Outcome{}, fmt.Errorf("sage model response malformed: %w", err)
	}
	var decoded response
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Outcome{}, fmt.Errorf("sage model response malformed: %w", err)
	}
	if decoded.WorthDocumenting == nil {
		return Outcome{}, errors.New("sage model response malformed: missing worth_documenting")
	}
	if !*decoded.WorthDocumenting {
		reason := flatten(decoded.SkipReason)
		if reason == "" {
			reason = "not worth documenting"
		}
		return Outcome{Skipped: true, SkipReason: reason}, nil
	}

	title := decoded.Title
	if strings.TrimSpace(title) == "" {
		title = decoded.Heading
	}
	draft := Draft{
		Title:   clean(title),
		Section: clean(decoded.Section),
		What:    clean(decoded.What),
		Why:     clean(decoded.Why),
		Example: clean(decoded.Example),
		Notes:   clean(decoded.Notes),
	}
	for _, command := range decoded.Commands {
		if command = strings.TrimSpace(signatureComment.ReplaceAllString(command, "")); command != "" {
			draft.Commands = append(draft.Commands, command)
		}
	}
	if len(draft.Commands) == 0 {
		draft.Commands = []string{strings.TrimSpace(capturedCommand)}
	}
	for name, value := range map[string]string{
		"title": draft.Title, "what": draft.What, "why": draft.Why, "example": draft.Example,
	} {
		if value == "" {
			return Outcome{}, fmt.Errorf("sage model response malformed: missing %s", name)
		}
	}
	return Outcome{Draft: &draft}, nil
}

func firstJSONObject(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(raw[i:]))
		var value json.RawMessage
		if err := decoder.Decode(&value); err == nil {
			return value, nil
		}
	}
	return nil, errors.New("response did not contain a JSON object")
}

func clean(value string) string {
	return flatten(signatureComment.ReplaceAllString(value, ""))
}

func flatten(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
