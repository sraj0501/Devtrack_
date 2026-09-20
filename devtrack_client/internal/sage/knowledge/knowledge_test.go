package knowledge

import (
	"strings"
	"testing"
	"time"
)

func TestTopicAndSearchExpressionAreDeterministic(t *testing.T) {
	if got := TopicForSignature("Git Status"); got != "git" {
		t.Fatalf("topic=%q", got)
	}
	if got := SearchExpression(`git status OR topic:secret "quoted"`); got != `"git" AND "status" AND "or" AND "topic" AND "secret" AND "quoted"` {
		t.Fatalf("expression=%q", got)
	}
}

func TestRenderEntriesIsStableAndSortsSources(t *testing.T) {
	entry := Entry{
		Signature: "git status", Topic: "git", Command: "git status --short",
		UseCount: 2, SuccessCount: 2, LastSeen: time.Unix(10, 0),
		SourceHarnesses: []string{"codex-history", "codex"},
	}
	first := RenderEntries([]Entry{entry})
	second := RenderEntries([]Entry{entry})
	if first != second {
		t.Fatal("rendering is not byte-stable")
	}
	if !strings.Contains(first, "Sources: `codex`, `codex-history`") || !strings.Contains(first, "    git status --short\n") {
		t.Fatalf("unexpected render:\n%s", first)
	}
}

func TestEmptyRenderingIsActionable(t *testing.T) {
	if got := RenderEntries(nil); got != "No Sage knowledge matched.\n" {
		t.Fatalf("entries=%q", got)
	}
	if got := RenderTopics(nil); got != "No Sage topics yet.\n" {
		t.Fatalf("topics=%q", got)
	}
}
