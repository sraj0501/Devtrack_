package routing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

type fakeClient struct {
	responses []string
	err       error
	calls     int
	messages  [][]llmclient.Message
}

func (f *fakeClient) ChatJSONContext(_ context.Context, messages []llmclient.Message) (string, error) {
	f.calls++
	f.messages = append(f.messages, append([]llmclient.Message(nil), messages...))
	if f.err != nil {
		return "", f.err
	}
	if len(f.responses) == 0 {
		return "", errors.New("no classifier response")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

func TestReferenceUnknownBinaryIsClassifiedOnceThenRemembered(t *testing.T) {
	root := t.TempDir()
	client := &fakeClient{responses: []string{`{"file":"package-managers.md","title":"Package managers"}`}}
	prepare := Preparer{Writer: knowledge.Writer{Root: root}, Client: client}.Prepare
	for index := 0; index < 2; index++ {
		job := distill.Job{Fact: distill.Fact{Command: "dnf install"}}
		if err := prepare(context.Background(), &job); err != nil {
			t.Fatal(err)
		}
		if job.Topic != "dnf" {
			t.Fatalf("topic=%q", job.Topic)
		}
	}
	if client.calls != 1 {
		t.Fatalf("classifier calls=%d", client.calls)
	}
	route, found, err := (knowledge.Writer{Root: root}).Route("dnf")
	if err != nil || !found || route.Filename != "package-managers.md" {
		t.Fatalf("route=%+v found=%v err=%v", route, found, err)
	}
}

func TestReferenceClassifierChoiceIsSanitised(t *testing.T) {
	root := t.TempDir()
	client := &fakeClient{responses: []string{`{"file":"../../etc/passwd","title":"unsafe"}`}}
	job := distill.Job{Fact: distill.Fact{Command: "nixos-rebuild switch"}}
	if err := (Preparer{Writer: knowledge.Writer{Root: root}, Client: client}).Prepare(context.Background(), &job); err != nil {
		t.Fatal(err)
	}
	route, found, err := (knowledge.Writer{Root: root}).Route("nixos-rebuild")
	if err != nil || !found {
		t.Fatalf("route=%+v found=%v err=%v", route, found, err)
	}
	if route.Filename != knowledge.SafeFilename("../../etc/passwd") || strings.ContainsAny(route.Filename, `/\\`) {
		t.Fatalf("unsafe route=%+v", route)
	}
	if _, err := os.Stat(filepath.Join(root, "..", "etc", "passwd")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("classifier escaped knowledge root: %v", err)
	}
}

func TestReferenceUnreachableClassifierFallsBackToItsOwnFile(t *testing.T) {
	root := t.TempDir()
	client := &fakeClient{err: errors.New("offline")}
	prepare := Preparer{Writer: knowledge.Writer{Root: root}, Client: client}.Prepare
	for index := 0; index < 2; index++ {
		job := distill.Job{Fact: distill.Fact{Command: "cargo test"}}
		if err := prepare(context.Background(), &job); err != nil {
			t.Fatal(err)
		}
	}
	if client.calls != 1 {
		t.Fatalf("classifier calls=%d", client.calls)
	}
	route, found, err := (knowledge.Writer{Root: root}).Route("cargo")
	if err != nil || !found || route.Filename != "cargo.md" {
		t.Fatalf("route=%+v found=%v err=%v", route, found, err)
	}
}

func TestPrepareOffersExistingTopicSections(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "git.md"), []byte("# Git\n\n## Inspecting state\n\n### Status\n<!-- sig: abcdef01 -->\n\n```bash\ngit status\n```\n"), 0600); err != nil {
		t.Fatal(err)
	}
	job := distill.Job{Fact: distill.Fact{Command: "git log"}}
	if err := (Preparer{Writer: knowledge.Writer{Root: root}}).Prepare(context.Background(), &job); err != nil {
		t.Fatal(err)
	}
	if len(job.Fact.ExistingSections) != 1 || job.Fact.ExistingSections[0] != "Inspecting state" {
		t.Fatalf("sections=%v", job.Fact.ExistingSections)
	}
}
