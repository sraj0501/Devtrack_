package sage

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	captureLogFilename  = "capture-decisions.ndjson"
	captureLogMaxBytes  = 512 * 1024
	captureLogKeepLines = 4000
)

var captureLogMu sync.Mutex

// CaptureLogEntry is deliberately content-free: it exposes an opaque action
// identifier and the capture decision, never the command or raw signature.
type CaptureLogEntry struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Harness string `json:"harness,omitempty"`
	AtMS    int64  `json:"at_ms"`
	Stage   string `json:"stage"`
}

func captureDecisionID(signature string) string {
	digest := sha256.Sum256([]byte(signature))
	return hex.EncodeToString(digest[:6])
}

// recordCaptureDecision is intentionally fail-open because it runs inside a
// harness tool loop. Capture and the host command must survive every log error.
func recordCaptureDecision(root, state string, event Event) {
	func() {
		defer func() { _ = recover() }()
		captureLogMu.Lock()
		defer captureLogMu.Unlock()

		if err := os.MkdirAll(root, 0700); err != nil {
			return
		}
		entry := CaptureLogEntry{
			ID: captureDecisionID(event.Signature), State: state,
			Harness: event.Harness, AtMS: time.Now().UTC().UnixMilli(), Stage: "capture",
		}
		raw, err := json.Marshal(entry)
		if err != nil {
			return
		}
		path := filepath.Join(root, captureLogFilename)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return
		}
		_, _ = file.Write(append(raw, '\n'))
		_ = file.Close()
		trimCaptureLog(path)
	}()
}

func trimCaptureLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < captureLogMaxBytes {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := splitLogLines(raw)
	if len(lines) > captureLogKeepLines {
		lines = lines[len(lines)-captureLogKeepLines:]
	}
	joined := make([]byte, 0, len(raw))
	for _, line := range lines {
		joined = append(joined, line...)
		joined = append(joined, '\n')
	}
	_ = os.WriteFile(path, joined, 0600)
}

func splitLogLines(raw []byte) [][]byte {
	var lines [][]byte
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 1024), MaxEventBytes)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		lines = append(lines, line)
	}
	return lines
}

// ReadCaptureLog returns the most recent valid capture decisions. Corrupt
// lines are ignored so diagnostics remain available after interrupted writes.
func ReadCaptureLog(root string, limit int) ([]CaptureLogEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	raw, err := os.ReadFile(filepath.Join(root, captureLogFilename))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := splitLogLines(raw)
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	entries := make([]CaptureLogEntry, 0, len(lines))
	for _, line := range lines {
		var entry CaptureLogEntry
		if json.Unmarshal(line, &entry) == nil && entry.Stage == "capture" &&
			(entry.State == "new" || entry.State == "known" || entry.State == "repeat") {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
