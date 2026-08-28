package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFirstRunGuidanceBeforeProfileIsReady(t *testing.T) {
	var out bytes.Buffer
	writeFirstRunGuidance(&out, nil, true)
	for _, want := range []string{
		"devtrack mcp setup",
		`What am I working on?`,
		"builds automatically",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("guidance missing %q:\n%s", want, out.String())
		}
	}
}

func TestFirstRunGuidanceAfterProfileIsReady(t *testing.T) {
	var out bytes.Buffer
	writeFirstRunGuidance(&out, &FirstRunResult{CommitCount: 64}, true)
	for _, want := range []string{"built from 64 commits", "devtrack work report"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("guidance missing %q:\n%s", want, out.String())
		}
	}
}

func TestFirstRunGuidanceKeepsLearningLocalInExternalMode(t *testing.T) {
	var out bytes.Buffer
	writeFirstRunGuidance(&out, nil, false)
	if got := out.String(); !strings.Contains(got, "available in managed mode") || strings.Contains(got, "builds automatically") {
		t.Fatalf("unexpected external-mode guidance:\n%s", got)
	}
}
