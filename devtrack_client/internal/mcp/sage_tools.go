package mcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/harness"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

// RegisterSageTools adds local read-only knowledge and diagnostics tools. Raw
// capture events, logs, prompts, model output, and mutation operations are not
// part of this surface.
func RegisterSageTools(s *Server, database *db.Database, root string, writer knowledge.Writer, adapters func() ([]harness.Descriptor, error)) {
	s.Register(Tool{
		Name: "sage_search", Title: "Search Sage knowledge", Annotations: readOnlyAnnotations(),
		Description: "Searches complete entries in the local deterministic Sage Markdown knowledge base. No model or network access is used.",
		InputSchema: map[string]any{"type": "object", "required": []string{"query"}, "additionalProperties": false, "properties": map[string]any{
			"query": map[string]any{"type": "string", "maxLength": 4096},
			"topic": map[string]any{"type": "string", "maxLength": 256},
			"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
		}},
		Handler: makeSageSearch(database, writer),
	})
	s.Register(Tool{
		Name: "sage_topics", Title: "List Sage topics", Annotations: readOnlyAnnotations(),
		Description: "Lists current local Sage Markdown topics and entry counts after reconciling edits, moves, and deletions.",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}},
		Handler:     makeSageTopics(database, writer),
	})
	s.Register(Tool{
		Name: "sage_get_entry", Title: "Get a Sage entry", Annotations: readOnlyAnnotations(),
		Description: "Returns one exact current Sage Markdown entry using the topic and line returned by sage_search.",
		InputSchema: map[string]any{"type": "object", "required": []string{"topic", "line"}, "additionalProperties": false, "properties": map[string]any{
			"topic": map[string]any{"type": "string", "maxLength": 256},
			"line":  map[string]any{"type": "integer", "minimum": 1},
		}},
		Handler: makeSageGetEntry(database, writer),
	})
	s.Register(Tool{
		Name: "sage_status", Title: "Get Sage status", Annotations: readOnlyAnnotations(),
		Description: "Returns local Sage capture, durable pipeline, and installed-adapter diagnostics without exposing raw activity.",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}},
		Handler:     makeSageStatus(database, root, adapters),
	})
}

func makeSageSearch(database *db.Database, writer knowledge.Writer) func(context.Context, map[string]any) (any, error) {
	return func(ctx context.Context, args map[string]any) (any, error) {
		if err := rejectUnknown(args, "query", "topic", "limit"); err != nil {
			return nil, err
		}
		query, err := requiredText(args, "query", 4096)
		if err != nil {
			return nil, err
		}
		topic, err := optionalText(args, "topic", 256)
		if err != nil {
			return nil, err
		}
		limit, err := optionalInteger(args, "limit", 20, 1, 100)
		if err != nil {
			return nil, err
		}
		entries, err := database.SearchSageEntries(ctx, writer, query, topic, limit)
		if err != nil {
			return nil, fmt.Errorf("sage_search: %w", err)
		}
		out := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			out = append(out, sageEntryPayload(entry))
		}
		return map[string]any{"entries": out, "count": len(out)}, nil
	}
}

func makeSageTopics(database *db.Database, writer knowledge.Writer) func(context.Context, map[string]any) (any, error) {
	return func(ctx context.Context, args map[string]any) (any, error) {
		if err := rejectUnknown(args); err != nil {
			return nil, err
		}
		topics, err := database.ListSageEntryTopics(ctx, writer)
		if err != nil {
			return nil, fmt.Errorf("sage_topics: %w", err)
		}
		out := make([]map[string]any, 0, len(topics))
		for _, topic := range topics {
			out = append(out, map[string]any{"name": topic.Name, "entries": topic.Entries})
		}
		return map[string]any{"topics": out, "count": len(out)}, nil
	}
}

func makeSageGetEntry(database *db.Database, writer knowledge.Writer) func(context.Context, map[string]any) (any, error) {
	return func(ctx context.Context, args map[string]any) (any, error) {
		if err := rejectUnknown(args, "topic", "line"); err != nil {
			return nil, err
		}
		topic, err := requiredText(args, "topic", 256)
		if err != nil {
			return nil, err
		}
		line, err := requiredInteger(args, "line", 1, math.MaxInt32)
		if err != nil {
			return nil, err
		}
		entry, err := database.GetSageEntry(ctx, writer, topic, line)
		if err != nil {
			return nil, fmt.Errorf("sage_get_entry: %w", err)
		}
		if entry == nil {
			return nil, errors.New("sage entry not found")
		}
		return map[string]any{"entry": sageEntryPayload(*entry)}, nil
	}
}

func makeSageStatus(database *db.Database, root string, adapters func() ([]harness.Descriptor, error)) func(context.Context, map[string]any) (any, error) {
	return func(ctx context.Context, args map[string]any) (any, error) {
		if err := rejectUnknown(args); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		state, err := sage.ReadState(root)
		if err != nil {
			return nil, errors.New("sage status unavailable")
		}
		pipeline, err := database.SageDiagnostics(ctx, time.Now())
		if err != nil {
			return nil, fmt.Errorf("sage_status: %w", err)
		}
		var installed []harness.Descriptor
		if adapters != nil {
			installed, err = adapters()
			if err != nil {
				return nil, errors.New("sage adapter diagnostics unavailable")
			}
		}
		return map[string]any{"capture": state, "pipeline": pipeline, "adapters": installed}, nil
	}
}

func sageEntryPayload(entry knowledge.Entry) map[string]any {
	return map[string]any{"topic": entry.Topic, "filename": entry.Filename, "line": entry.Line, "heading": entry.Heading, "body": entry.Body, "command": entry.Command}
}

func requiredText(args map[string]any, key string, max int) (string, error) {
	value, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	if len(text) > max {
		return "", fmt.Errorf("%s exceeds size limit", key)
	}
	return strings.TrimSpace(text), nil
}

func optionalText(args map[string]any, key string, max int) (string, error) {
	value, ok := args[key]
	if !ok {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	if len(text) > max {
		return "", fmt.Errorf("%s exceeds size limit", key)
	}
	return strings.TrimSpace(text), nil
}

func optionalInteger(args map[string]any, key string, fallback, min, max int) (int, error) {
	if _, ok := args[key]; !ok {
		return fallback, nil
	}
	return requiredInteger(args, key, min, max)
}

func requiredInteger(args map[string]any, key string, min, max int) (int, error) {
	value, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	if math.Trunc(number) != number || number < float64(min) || number > float64(max) {
		return 0, fmt.Errorf("%s is out of range", key)
	}
	return int(number), nil
}

func rejectUnknown(args map[string]any, allowed ...string) error {
	set := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		set[key] = true
	}
	for key := range args {
		if !set[key] {
			return fmt.Errorf("unexpected argument: %s", key)
		}
	}
	return nil
}
