// Package routing classifies previously unseen command binaries into stable
// Sage knowledge topics. It runs only in the daemon-owned background worker.
package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/command"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

type Client interface {
	ChatJSONContext(context.Context, []llmclient.Message) (string, error)
}

type Preparer struct {
	Writer  knowledge.Writer
	Client  Client
	Timeout time.Duration
}

type classifierResponse struct {
	File  string `json:"file"`
	Title string `json:"title"`
}

// Prepare gives one durable job a stable topic and the headings already in
// that topic. Classifier errors deliberately fall back to the binary's own
// file and that decision is remembered, so an unavailable model is not asked
// again forever.
func (p Preparer) Prepare(ctx context.Context, job *distill.Job) error {
	if job == nil {
		return errors.New("sage routing job is nil")
	}
	actions := command.Parse(job.Fact.Command)
	if len(actions) == 0 {
		return errors.New("sage routing fact has no command action")
	}
	binary := actions[0].Binary
	route, found, err := p.Writer.Route(binary)
	if err != nil {
		return fmt.Errorf("read Sage route: %w", err)
	}
	if !found {
		filename := p.classify(ctx, actions[0])
		if err := p.Writer.RememberRoute(binary, filename); err != nil {
			return fmt.Errorf("remember Sage route: %w", err)
		}
		route, found, err = p.Writer.Route(binary)
		if err != nil {
			return fmt.Errorf("read remembered Sage route: %w", err)
		}
		if !found {
			return errors.New("remembered Sage route is unavailable")
		}
	}
	sections, err := p.Writer.Sections(route.Filename)
	if err != nil {
		return fmt.Errorf("read Sage topic sections: %w", err)
	}
	job.Topic = binary
	job.Fact.ExistingSections = sections
	return nil
}

func (p Preparer) classify(ctx context.Context, action command.Action) string {
	fallback := knowledge.SafeFilename(action.Binary)
	if p.Client == nil {
		return fallback
	}
	routes, err := p.Writer.Routes()
	if err != nil {
		return fallback
	}
	type candidate struct {
		File  string `json:"file"`
		Title string `json:"title"`
	}
	candidates := make([]candidate, 0, len(routes))
	for _, route := range routes {
		candidates = append(candidates, candidate{File: route.Filename, Title: route.Title})
	}
	payload, _ := json.Marshal(struct {
		Binary     string      `json:"binary"`
		Command    string      `json:"command"`
		Candidates []candidate `json:"existing_topics"`
	}{Binary: action.Binary, Command: action.Command, Candidates: candidates})
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, err := p.Client.ChatJSONContext(requestCtx, []llmclient.Message{
		{Role: "system", Content: "You organise a personal command reference. Choose an existing topic when it fits, otherwise propose a portable Markdown filename. Return one JSON object with file and title only."},
		{Role: "user", Content: string(payload)},
	})
	if err != nil {
		return fallback
	}
	body, err := firstJSONObject(raw)
	if err != nil {
		return fallback
	}
	var response classifierResponse
	if json.Unmarshal(body, &response) != nil || strings.TrimSpace(response.File) == "" {
		return fallback
	}
	return knowledge.SafeFilename(response.File)
}

func firstJSONObject(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	for index := 0; index < len(raw); index++ {
		if raw[index] != '{' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(raw[index:]))
		var value json.RawMessage
		if err := decoder.Decode(&value); err == nil {
			return value, nil
		}
	}
	return nil, errors.New("classifier response did not contain JSON")
}
