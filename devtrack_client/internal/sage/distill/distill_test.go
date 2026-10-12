package distill

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
)

func TestParseAcceptsFencedChatterAndCleansFields(t *testing.T) {
	raw := "Here is the entry:\n```json\n" + `{
        "worth_documenting": true,
        "title": "Inspect   repository\nstate <!-- sig: abcdef123456 -->",
        "section": "Inspecting state",
        "commands": [],
        "what": "Shows the current\nworking tree.",
        "why": "Checks changes before committing.",
        "example": "git status --short",
        "notes": ""
    }` + "\n```"
	outcome, err := Parse(raw, "git status --short")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Draft == nil || outcome.Draft.Title != "Inspect repository state" {
		t.Fatalf("outcome=%#v", outcome)
	}
	if got := outcome.Draft.Commands; len(got) != 1 || got[0] != "git status --short" {
		t.Fatalf("commands=%q", got)
	}
	if strings.Contains(outcome.Draft.What, "\n") {
		t.Fatalf("what=%q", outcome.Draft.What)
	}
}

func TestParseDistinguishesSkipFromMalformed(t *testing.T) {
	skip, err := Parse(`{"worth_documenting":false,"skip_reason":"prints the directory"}`, "pwd")
	if err != nil || !skip.Skipped || skip.SkipReason != "prints the directory" {
		t.Fatalf("skip=%#v err=%v", skip, err)
	}
	if _, err := Parse(`{"worth_documenting":true,"title":"Missing prose"}`, "git status"); err == nil {
		t.Fatal("malformed response was accepted")
	}
}

type fakeClient struct {
	response string
	err      error
	wait     bool
}

func (f fakeClient) ChatJSONContext(ctx context.Context, _ []llmclient.Message) (string, error) {
	if f.wait {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return f.response, f.err
}

func TestDistillerMarksModelFailuresRetryable(t *testing.T) {
	service := Distiller{Client: fakeClient{err: errors.New("offline")}}
	_, err := service.Distill(context.Background(), Fact{Command: "git status"})
	if !IsRetryable(err) || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("err=%v", err)
	}
}

func TestDistillerTimeoutIsRetryable(t *testing.T) {
	service := Distiller{Client: fakeClient{wait: true}, Timeout: 5 * time.Millisecond}
	_, err := service.Distill(context.Background(), Fact{Command: "git status"})
	if !IsRetryable(err) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestDistillerReturnsExplicitSkipWithoutRetry(t *testing.T) {
	service := Distiller{Client: fakeClient{response: `{"worth_documenting":false,"skip_reason":"trivial"}`}}
	outcome, err := service.Distill(context.Background(), Fact{Command: "pwd"})
	if err != nil || !outcome.Skipped || outcome.SkipReason != "trivial" {
		t.Fatalf("outcome=%#v err=%v", outcome, err)
	}
}

// A reasoning model may exhaust its budget without producing an answer. That
// is a retryable transport failure, never a decision to discard captured work.
func TestDistillerEmptyModelOutputRemainsRetryable(t *testing.T) {
	for _, provider := range []string{"ollama", "openai"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if provider == "ollama" {
					_, _ = w.Write([]byte(`{"message":{"content":""},"done":true,"done_reason":"length"}`))
				} else {
					_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`))
				}
			}))
			defer server.Close()
			service := Distiller{Client: llmclient.Config{Host: server.URL, Provider: provider, Model: "test", Client: server.Client()}}
			outcome, err := service.Distill(context.Background(), Fact{Command: "git status"})
			if !IsRetryable(err) || outcome.Skipped || outcome.Draft != nil {
				t.Fatalf("empty model output became a verdict: outcome=%#v err=%v", outcome, err)
			}
		})
	}
}
