// Package sage defines the local, versioned DevTrack Sage capture contract.
// Harness-specific payloads are normalized before they reach this package.
package sage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

const SchemaVersion = 1
const MaxEventBytes = 16 * 1024

var identifier = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var signature = regexp.MustCompile(`^[a-z0-9][a-z0-9 _.-]{0,127}$`)
var unsafeCommand = regexp.MustCompile(`(?i)(?:password|passwd|api[_-]?key|secret|token|authorization|bearer|ghp_|github_pat_|sk-[a-z0-9]|(?:[a-z]:\\users\\|/home/|/users/))`)

// Event is a deliberately small fact, not a transcript. Command must already
// be reduced to a safe, displayable form by the harness adapter. Unknown
// fields, including raw output and reasoning, are never accepted.
type Event struct {
	SchemaVersion int       `json:"schema_version"`
	EventID       string    `json:"event_id"`
	Harness       string    `json:"harness"`
	SessionID     string    `json:"session_id"`
	EventType     string    `json:"event_type"`
	Tool          string    `json:"tool"`
	OccurredAt    time.Time `json:"occurred_at"`
	ProjectID     string    `json:"project_id,omitempty"`
	Command       string    `json:"command"`
	Signature     string    `json:"signature"`
	Success       *bool     `json:"success,omitempty"`
	ExitCode      *int      `json:"exit_code,omitempty"`
}

// DecodeEvent validates an adapter-normalized event without ever including
// potentially sensitive input in an error. It has no filesystem or network I/O.
func DecodeEvent(raw []byte) (Event, error) {
	var event Event
	if len(raw) == 0 || len(raw) > MaxEventBytes {
		return event, errors.New("sage event size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return Event{}, errors.New("sage event JSON is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Event{}, errors.New("sage event has trailing data")
	}
	if event.SchemaVersion != SchemaVersion ||
		!identifier.MatchString(event.EventID) ||
		!identifier.MatchString(event.Harness) ||
		!identifier.MatchString(event.SessionID) ||
		(event.ProjectID != "" && !identifier.MatchString(event.ProjectID)) ||
		event.EventType != "command" ||
		!identifier.MatchString(event.Tool) ||
		event.OccurredAt.IsZero() ||
		len(event.Command) == 0 || len(event.Command) > 4096 ||
		strings.ContainsAny(event.Command, "\r\n\x00") ||
		unsafeCommand.MatchString(event.Command) ||
		!signature.MatchString(event.Signature) ||
		unsafeCommand.MatchString(event.Signature) {
		return Event{}, errors.New("sage event contract is invalid")
	}
	if event.ExitCode != nil && event.Success != nil && (*event.ExitCode == 0) != *event.Success {
		return Event{}, errors.New("sage event exit status conflicts with success")
	}
	return event, nil
}

// DeliveryKey identifies repeated deliveries of one harness event. The
// importer will enforce this key durably in SAGE-002.
func (event Event) DeliveryKey() string {
	sum := sha256.Sum256([]byte(event.Harness + "\x00" + event.SessionID + "\x00" + event.EventID))
	return hex.EncodeToString(sum[:])
}
