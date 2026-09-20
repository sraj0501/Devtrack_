// Package codexhistory provides the opt-in, read-only compatibility adapter
// for Codex clients whose PostToolUse hook is not delivered.
package codexhistory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/hooks"
	_ "modernc.org/sqlite"
)

const stateFilename = "codex-history-state.json"
const defaultRowLimit = 500

type progress struct {
	Ordinal int64    `json:"ordinal"`
	Pending []string `json:"pending"`
}

type pollState struct {
	SinceMS  int64               `json:"since_ms"`
	Threads  map[string]progress `json:"threads"`
	LastPoll int64               `json:"last_poll_ms,omitempty"`
	CutoffMS int64               `json:"cutoff_ms,omitempty"`
}

type Poller struct {
	Root      string
	CodexHome string
	Now       func() time.Time
	RowLimit  int
}

func Enabled(root string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEVTRACK_SAGE_CODEX_HISTORY"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return sage.CodexHistoryEnabled(root)
	}
}

func DefaultCodexHome() (string, error) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

// Poll reads only completed commandExecution projections and spools normalized
// facts. On first use it starts from now and does not backfill older sessions.
func (p Poller) Poll(ctx context.Context) (int, error) {
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.RowLimit <= 0 || p.RowLimit > defaultRowLimit {
		p.RowLimit = defaultRowLimit
	}
	statePath := filepath.Join(p.Root, stateFilename)
	state, err := loadState(statePath)
	if errors.Is(err, os.ErrNotExist) {
		cutoff, _ := sage.CaptureCutoff(p.Root)
		state = pollState{SinceMS: p.Now().UnixMilli(), CutoffMS: cutoff, Threads: map[string]progress{}}
		return 0, saveState(statePath, state)
	}
	if err != nil {
		return 0, errors.New("codex history state is invalid")
	}
	cutoff, err := sage.CaptureCutoff(p.Root)
	if err != nil {
		return 0, err
	}
	if cutoff > state.CutoffMS {
		state.SinceMS = cutoff
		state.CutoffMS = cutoff
		state.Threads = map[string]progress{}
	}
	sageState, err := sage.ReadState(p.Root)
	if err != nil {
		return 0, err
	}
	if sageState.Paused {
		return 0, nil
	}
	if p.CodexHome == "" {
		p.CodexHome, err = DefaultCodexHome()
		if err != nil {
			return 0, errors.New("codex home unavailable")
		}
	}
	index, err := openReadOnly(filepath.Join(p.CodexHome, "state_5.sqlite"))
	if err != nil {
		return 0, errors.New("codex history index unavailable")
	}
	defer index.Close()
	history, err := openReadOnly(filepath.Join(p.CodexHome, "thread_history_1.sqlite"))
	if err != nil {
		return 0, errors.New("codex command history unavailable")
	}
	defer history.Close()

	rows, err := index.QueryContext(ctx, `SELECT id, source FROM threads WHERE source IN ('vscode','cli') AND archived = 0`)
	if err != nil {
		return 0, errors.New("codex history schema unsupported")
	}
	defer rows.Close()
	type thread struct{ id, source string }
	var threads []thread
	for rows.Next() {
		var item thread
		if err := rows.Scan(&item.id, &item.source); err != nil {
			return 0, errors.New("codex history row invalid")
		}
		threads = append(threads, item)
	}
	if err := rows.Err(); err != nil {
		return 0, errors.New("codex history read failed")
	}

	count := 0
	for _, thread := range threads {
		threadProgress, ok := state.Threads[thread.id]
		if !ok {
			threadProgress = progress{Ordinal: -1}
		}
		captured, next, err := p.pollThread(ctx, history, state.SinceMS, thread.id, thread.source, threadProgress)
		if err != nil {
			return count, err
		}
		count += captured
		state.Threads[thread.id] = next
	}
	state.LastPoll = p.Now().UnixMilli()
	if err := saveState(statePath, state); err != nil {
		return count, err
	}
	return count, nil
}

func (p Poller) pollThread(ctx context.Context, history *sql.DB, since int64, threadID, source string, current progress) (int, progress, error) {
	type row struct {
		itemID           string
		ordinal, created int64
		raw              []byte
	}
	var items []row
	rows, err := history.QueryContext(ctx, `SELECT item_id, rollout_ordinal, created_at_ms, item_json FROM thread_items WHERE thread_id=? AND item_type='commandExecution' AND created_at_ms>=? AND rollout_ordinal>? ORDER BY rollout_ordinal LIMIT ?`, threadID, since, current.Ordinal, p.RowLimit)
	if err != nil {
		return 0, current, errors.New("codex command history schema unsupported")
	}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.itemID, &item.ordinal, &item.created, &item.raw); err != nil {
			rows.Close()
			return 0, current, errors.New("codex command history row invalid")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, current, errors.New("codex command history read failed")
	}
	if err := rows.Close(); err != nil {
		return 0, current, errors.New("codex command history read failed")
	}
	for _, itemID := range current.Pending {
		var item row
		err := history.QueryRowContext(ctx, `SELECT item_id, rollout_ordinal, created_at_ms, item_json FROM thread_items WHERE thread_id=? AND item_id=?`, threadID, itemID).Scan(&item.itemID, &item.ordinal, &item.created, &item.raw)
		if err == nil {
			items = append(items, item)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return 0, current, errors.New("codex pending history read failed")
		}
	}
	pending := make(map[string]bool, len(current.Pending))
	for _, id := range current.Pending {
		pending[id] = true
	}
	visited := map[string]bool{}
	count := 0
	for _, item := range items {
		if visited[item.itemID] {
			continue
		}
		visited[item.itemID] = true
		if item.ordinal > current.Ordinal {
			current.Ordinal = item.ordinal
		}
		var status struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(item.raw, &status) != nil {
			return count, current, errors.New("codex command history JSON invalid")
		}
		if status.Status != "completed" && status.Status != "failed" && status.Status != "declined" && status.Status != "cancelled" {
			pending[item.itemID] = true
			continue
		}
		delete(pending, item.itemID)
		for _, event := range hooks.NormalizeCodexHistoryItem(item.raw, source, threadID, item.itemID, time.UnixMilli(item.created).UTC()) {
			if err := sage.WriteEvent(p.Root, event); err != nil {
				return count, current, err
			}
			count++
		}
	}
	current.Pending = current.Pending[:0]
	for id := range pending {
		current.Pending = append(current.Pending, id)
	}
	sort.Strings(current.Pending)
	return count, current, nil
}

func openReadOnly(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	abs = filepath.ToSlash(abs)
	if filepath.VolumeName(abs) != "" && !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	location := &url.URL{Scheme: "file", Path: abs}
	query := location.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", "busy_timeout(2000)")
	location.RawQuery = query.Encode()
	database, err := sql.Open("sqlite", location.String())
	if err != nil {
		return nil, err
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

func loadState(path string) (pollState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return pollState{}, err
	}
	var state pollState
	if json.Unmarshal(raw, &state) != nil || state.SinceMS <= 0 || state.Threads == nil {
		return pollState{}, errors.New("invalid state")
	}
	return state, nil
}

func saveState(path string, state pollState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("codex history state unavailable")
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".history-state-*.tmp")
	if err != nil {
		return errors.New("codex history state unavailable")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("save codex history state: %w", err)
	}
	return nil
}
