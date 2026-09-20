package sage

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDecodeEventV1Fixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/command-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	event, err := DecodeEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if event.SchemaVersion != SchemaVersion || event.Signature != "git status" || event.Success == nil || !*event.Success {
		t.Fatalf("unexpected event: %+v", event)
	}
	if len(event.DeliveryKey()) != 64 {
		t.Fatal("delivery key has wrong format")
	}
}

func TestDecodeEventRejectsUnsafeInput(t *testing.T) {
	fixture, err := os.ReadFile("testdata/command-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"empty":              nil,
		"oversized":          bytes.Repeat([]byte("x"), MaxEventBytes+1),
		"malformed":          []byte(`{"schema_version":`),
		"trailing":           append(append([]byte{}, fixture...), []byte(` {}`)...),
		"unknown field":      bytes.Replace(fixture, []byte(`"command":`), []byte(`"raw_output":"private", "command":`), 1),
		"wrong version":      bytes.Replace(fixture, []byte(`"schema_version": 1`), []byte(`"schema_version": 2`), 1),
		"secret":             bytes.Replace(fixture, []byte(`git status --short`), []byte(`curl --token=private-value`), 1),
		"private path":       bytes.Replace(fixture, []byte(`git status --short`), []byte(`ls /home/alice/project`), 1),
		"secret signature":   bytes.Replace(fixture, []byte(`git status"`), []byte(`api_key"`), 1),
		"bad project id":     bytes.Replace(fixture, []byte(`project-001`), []byte(`/home/alice/project`), 1),
		"conflicting status": bytes.Replace(fixture, []byte(`"exit_code": 0`), []byte(`"exit_code": 1`), 1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeEvent(raw)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "alice") {
				t.Fatalf("error leaked input: %v", err)
			}
		})
	}
}

func TestDeliveryKeyDeduplicatesOneEventNotDistinctEvents(t *testing.T) {
	fixture, err := os.ReadFile("testdata/command-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := DecodeEvent(fixture)
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecodeEvent(bytes.Replace(fixture, []byte(`evt-001`), []byte(`evt-002`), 1))
	if err != nil {
		t.Fatal(err)
	}
	if a.DeliveryKey() == b.DeliveryKey() {
		t.Fatal("different event IDs collided")
	}
	repeated, err := DecodeEvent(fixture)
	if err != nil || a.DeliveryKey() != repeated.DeliveryKey() {
		t.Fatal("duplicate event changed delivery key")
	}
}
