package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/hooks"
	sageknowledge "github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

func TestRouteSageRejectsLegacyAndUnknownCommands(t *testing.T) {
	for _, tc := range []struct {
		args []string
		sub  string
		rest []string
	}{
		{[]string{"status"}, "status", nil},
		{[]string{"log"}, "log", nil},
		{[]string{"search", "git"}, "search", []string{"git"}},
		{[]string{"routes"}, "routes", nil},
		{[]string{"merge", "grep", "shell"}, "merge", []string{"grep", "shell"}},
		{[]string{"route", "git", "version-control"}, "route", []string{"git", "version-control"}},
		{[]string{"harness", "list"}, "harness", []string{"list"}},
		{[]string{"hook", "codex"}, "hook", []string{"codex"}},
	} {
		sub, rest, err := routeSage(tc.args)
		if err != nil || sub != tc.sub || len(rest) != len(tc.rest) {
			t.Fatalf("routeSage(%q) = %q, %q, %v", tc.args, sub, rest, err)
		}
		for i := range rest {
			if rest[i] != tc.rest[i] {
				t.Fatalf("routeSage(%q) rest = %q", tc.args, rest)
			}
		}
	}

	for _, args := range [][]string{
		nil,
		{"ask", "why"},
		{"do", "task"},
		{"pr"},
		{"interactive"},
		{"git", "ask", "why"},
		{"what", "changed"},
		{"statsu"},
	} {
		if _, _, err := routeSage(args); err == nil {
			t.Fatalf("routeSage(%q) accepted a removed or unknown command", args)
		}
	}
}

func TestReferenceStopAndStartRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var output bytes.Buffer
	if err := writeSageState("stop", []string{"testing"}, &output); err != nil {
		t.Fatal(err)
	}
	root, err := sageRoot()
	if err != nil {
		t.Fatal(err)
	}
	state, err := sage.ReadState(root)
	if err != nil || !state.Paused {
		t.Fatalf("stopped state=%+v err=%v", state, err)
	}
	output.Reset()
	if err := writeSageState("start", nil, &output); err != nil {
		t.Fatal(err)
	}
	state, err = sage.ReadState(root)
	if err != nil || state.Paused {
		t.Fatalf("started state=%+v err=%v", state, err)
	}
}

func TestReferenceRestartReplacesTheWatcherAndStopReleasesIt(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root, err := sageRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sage.SetPaused(root, true); err != nil {
		t.Fatal(err)
	}
	before, err := sage.CaptureCutoff(root)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := writeSageState("restart", nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	state, err := sage.ReadState(root)
	after, cutoffErr := sage.CaptureCutoff(root)
	if err != nil || cutoffErr != nil || state.Paused || after <= before {
		t.Fatalf("restart state=%+v cutoff=%d -> %d err=%v/%v", state, before, after, err, cutoffErr)
	}
	if _, err := os.Stat(filepath.Join(root, "watcher.pid")); !os.IsNotExist(err) {
		t.Fatalf("Sage restart created a second watcher: %v", err)
	}
	if err := writeSageState("stop", nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	state, err = sage.ReadState(root)
	if err != nil || !state.Paused {
		t.Fatalf("stop state=%+v err=%v", state, err)
	}
}

func TestReferenceAliasesAllWork(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root, err := sageRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, stop := range []string{"stop", "pause", "off"} {
		if err := writeSageState(stop, nil, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		state, err := sage.ReadState(root)
		if err != nil || !state.Paused {
			t.Fatalf("%s state=%+v err=%v", stop, state, err)
		}
		if err := writeSageState("start", nil, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, start := range []string{"start", "resume", "on"} {
		if err := writeSageState("stop", nil, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if err := writeSageState(start, nil, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		state, err := sage.ReadState(root)
		if err != nil || state.Paused {
			t.Fatalf("%s state=%+v err=%v", start, state, err)
		}
	}
}

func TestReferenceATypoIsLoudRatherThanASilentNoOp(t *testing.T) {
	if _, _, err := routeSage([]string{"nonsense"}); err == nil || !strings.Contains(err.Error(), "unknown Sage command") {
		t.Fatalf("unknown action error=%v", err)
	}
}

func TestReferenceStopRecordsItsReason(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := writeSageState("stop", []string{"reviewing", "a", "client", "repo"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root, err := sageRoot()
	if err != nil {
		t.Fatal(err)
	}
	state, err := sage.ReadState(root)
	if err != nil || state.PauseReason != "reviewing a client repo" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestReferenceHookVerbsAreSilentAndNeverFail(t *testing.T) {
	for _, verb := range []string{"hook-start", "hook-end"} {
		var output bytes.Buffer
		if err := writeSageState(verb, []string{"devin"}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s output=%q err=%v", verb, output.String(), err)
		}
	}
}

func TestReferenceALegacyShimCannotSilentlyRestartCapture(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := writeSageState("stop", nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeSageState("start", []string{"devin"}, &output); err != nil || output.Len() != 0 {
		t.Fatalf("legacy shim output=%q err=%v", output.String(), err)
	}
	root, err := sageRoot()
	if err != nil {
		t.Fatal(err)
	}
	state, err := sage.ReadState(root)
	if err != nil || !state.Paused {
		t.Fatalf("legacy shim resumed capture: state=%+v err=%v", state, err)
	}
}

func TestReferenceStatusReportsBothStates(t *testing.T) {
	root := t.TempDir()
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := os.MkdirAll(filepath.Join(root, "spool", "pending"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "spool", "pending", "captured.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB().Exec(`INSERT INTO sage_jobs(signature,command,failed) VALUES('status','private',0)`); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeSageDiagnostics(database, root, "status", nil, &output, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	var status sageStatus
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Backlog != 1 || status.Pipeline.Waiting != 1 || status.Healthy != nil {
		t.Fatalf("status=%+v", status)
	}
	output.Reset()
	if err := writeSageDiagnostics(database, root, "doctor", nil, &output, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &status); err != nil || status.Healthy == nil || !*status.Healthy {
		t.Fatalf("doctor=%s err=%v", output.String(), err)
	}
	output.Reset()
	if err := writeSageDiagnostics(database, root, "log", nil, &output, time.Unix(100, 0)); err != nil || !strings.Contains(output.String(), `"state":"waiting"`) {
		t.Fatalf("log=%q err=%v", output.String(), err)
	}
	if err := writeSageDiagnostics(database, root, "status", []string{"unexpected"}, &output, time.Unix(100, 0)); err == nil {
		t.Fatal("unexpected argument accepted")
	}
}

func TestReferenceLogRecordsADecisionPerLine(t *testing.T) {
	root := t.TempDir()
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, signature := range []string{"private first", "private second"} {
		if _, err := database.DB().Exec(`INSERT INTO sage_jobs
			(signature,command,failed,state,attempts,last_error,skip_reason)
			VALUES(?,?,0,'retrying',1,'CANARY\nprivate error','CANARY skip')`, signature, signature); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := writeSageDiagnostics(database, root, "log", nil, &output, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || strings.Contains(output.String(), "CANARY") || strings.Contains(output.String(), "private") {
		t.Fatalf("unsafe log=%q", output.String())
	}
	for _, line := range lines {
		var entry db.SageLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid NDJSON line %q: %v", line, err)
		}
		if entry.Diagnostic != "processing failure" || entry.SkipReason != "explicit skip" {
			t.Fatalf("unsanitized entry=%+v", entry)
		}
	}
}

func TestReferenceLogFlattensNewlinesSoOneActionIsOneLine(t *testing.T) {
	TestReferenceLogRecordsADecisionPerLine(t)
}

func TestSageLogIncludesPrivacySafeCaptureDecisions(t *testing.T) {
	root := t.TempDir()
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	success, exitCode := true, 0
	event := sage.Event{
		SchemaVersion: sage.SchemaVersion, EventID: "capture-log", Harness: "codex",
		SessionID: "session", EventType: "command", Tool: "shell",
		OccurredAt: time.Unix(10, 0).UTC(), Command: "git status --short",
		Signature: "git status --short", Success: &success, ExitCode: &exitCode,
	}
	if err := sage.WriteEvent(root, event); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeSageDiagnostics(database, root, "log", nil, &output, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), event.Command) || strings.Contains(output.String(), event.Signature) {
		t.Fatalf("capture content leaked: %q", output.String())
	}
	var entry sage.CaptureLogEntry
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry); err != nil || entry.State != "new" || entry.Stage != "capture" || entry.ID == "" {
		t.Fatalf("entry=%+v output=%q err=%v", entry, output.String(), err)
	}
}

func TestSageRoutesCLIUsesConfiguredKnowledgeAndValidatesBeforeOpeningDatabase(t *testing.T) {
	for _, args := range [][]string{nil, {"source"}, {"source", ""}, {"a", "b", "c"}} {
		var output bytes.Buffer
		if err := runSageRoutes("merge", args, &output); err == nil || output.Len() != 0 {
			t.Fatalf("invalid merge accepted: %v %v", args, err)
		}
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("DEVTRACK_SAGE_KNOWLEDGE_DIR", root)
	w := sageknowledge.Writer{Root: root}
	if err := w.RememberRoute("git", "version-control"); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runSageRoutes("routes", nil, &output); err != nil {
		t.Fatal(err)
	}
	var routes []sageknowledge.Route
	if err := json.Unmarshal(output.Bytes(), &routes); err != nil || len(routes) != 1 || routes[0].Filename != "version-control.md" || routes[0].Source != "remembered" {
		t.Fatalf("%s %v", output.String(), err)
	}
	for _, tc := range []struct {
		sub  string
		args []string
	}{
		{"routes", []string{"extra"}}, {"route", nil}, {"route", []string{"git"}},
		{"route", []string{"git", "topic", "extra"}}, {"route", []string{"../bad", "topic"}},
	} {
		output.Reset()
		if err := runSageRoutes(tc.sub, tc.args, &output); err == nil || output.Len() != 0 {
			t.Fatalf("accepted %s %v", tc.sub, tc.args)
		}
	}
}

func TestSageCLIRuntimeOverrideDoesNotMoveKnowledgeRoot(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("DEVTRACK_SAGE_RUNTIME_DIR", ".runtime")
	t.Setenv("DEVTRACK_SAGE_KNOWLEDGE_DIR", "")

	base := filepath.Join(dataHome, "devtrack", "sage")
	runtimeRoot, err := sageRoot()
	if err != nil {
		t.Fatal(err)
	}
	knowledgeRoot, err := sageKnowledgeRoot()
	if err != nil {
		t.Fatal(err)
	}
	if runtimeRoot != filepath.Join(base, ".runtime") {
		t.Fatalf("runtime root = %q", runtimeRoot)
	}
	if knowledgeRoot != filepath.Join(base, "knowledge") {
		t.Fatalf("knowledge root = %q", knowledgeRoot)
	}
}

func TestPackagedCodexHookPathWritesOneSilentSpoolEvent(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	payload := `{"session_id":"thread","cwd":"C:/private/project","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"call","tool_input":{"command":"git status --short"},"tool_response":"CANARY"}`
	var output bytes.Buffer
	if err := runSageHook([]string{"codex", "--devtrack-sage-hook"}, bytes.NewBufferString(payload)); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("hook wrote output: %q", output.String())
	}
	entries, err := os.ReadDir(filepath.Join(dataHome, "devtrack", "sage", "spool", "pending"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("spool files=%d err=%v", len(entries), err)
	}
	raw, err := os.ReadFile(filepath.Join(dataHome, "devtrack", "sage", "spool", "pending", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("CANARY")) || bytes.Contains(raw, []byte("C:/private")) {
		t.Fatalf("private payload leaked: %s", raw)
	}
}

func TestReferenceEnvSwitchIsProcessLocal(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	payload := `{"session_id":"thread","cwd":"C:/private/project","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"call","tool_input":{"command":"git status --short"},"tool_response":"CANARY"}`
	t.Setenv("DEVTRACK_SAGE_CAPTURE_OFF", "1")
	if err := runSageHook([]string{"codex", hooks.OwnerMarker}, strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dataHome, "devtrack", "sage")
	state, err := sage.ReadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.Paused {
		t.Fatal("process-local capture switch changed persistent pause state")
	}
	entries, err := os.ReadDir(filepath.Join(root, "spool", "pending"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("capture-off process wrote %d event(s)", len(entries))
	}

	t.Setenv("DEVTRACK_SAGE_CAPTURE_OFF", "")
	if err := runSageHook([]string{"codex", hooks.OwnerMarker}, strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(filepath.Join(root, "spool", "pending"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("capture after process-local switch: entries=%d err=%v", len(entries), err)
	}
}

func TestReferencePausedStopsCapture(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("CODEX_HOME", t.TempDir())
	var installOutput bytes.Buffer
	if err := runSageHarness([]string{"install", "codex"}, &installOutput); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dataHome, "devtrack", "sage")
	if _, err := sage.SetPaused(root, true); err != nil {
		t.Fatal(err)
	}
	payload := `{"session_id":"thread","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"paused-call","tool_input":{"command":"git status --short"}}`
	if err := runSageHook([]string{"codex", hooks.OwnerMarker}, strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "spool", "pending"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("paused installed adapter wrote %d event(s)", len(entries))
	}
	logEntries, err := sage.ReadCaptureLog(root, 10)
	if err != nil || len(logEntries) != 0 {
		t.Fatalf("paused capture log=%+v err=%v", logEntries, err)
	}
}

func TestReferenceRepeatWithinOneCommandLineIsMarked(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	payload := `{"session_id":"thread","hook_event_name":"PostToolUse","tool_name":"Bash","tool_use_id":"compound-call","tool_input":{"command":"tail -5 && tail -5"}}`
	if err := runSageHook([]string{"codex", hooks.OwnerMarker}, strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dataHome, "devtrack", "sage")
	entries, err := sage.ReadCaptureLog(root, 10)
	if err != nil || len(entries) != 2 || entries[0].State != "new" || entries[1].State != "repeat" || entries[0].ID != entries[1].ID {
		t.Fatalf("capture decisions=%+v err=%v", entries, err)
	}
	files, err := os.ReadDir(filepath.Join(root, "spool", "pending"))
	if err != nil || len(files) != 1 {
		t.Fatalf("pending=%d err=%v", len(files), err)
	}
}

func TestSelectiveHarnessCLIListsInstallsAndRemovesCodex(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	var output bytes.Buffer
	if err := runSageHarness([]string{"list"}, &output); err != nil {
		t.Fatal(err)
	}
	var listed []struct {
		ID        string `json:"id"`
		Installed bool   `json:"installed"`
	}
	if err := json.Unmarshal(output.Bytes(), &listed); err != nil || len(listed) != 1 || listed[0].ID != "codex" || listed[0].Installed {
		t.Fatalf("list=%+v err=%v", listed, err)
	}
	output.Reset()
	if err := runSageHarness([]string{"install", "codex"}, &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := runSageHarness([]string{"list"}, &output); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &listed); err != nil || !listed[0].Installed {
		t.Fatalf("installed list=%+v err=%v", listed, err)
	}
	if err := runSageHarness([]string{"uninstall", "codex"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := runSageHarness([]string{"install", "missing"}, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown harness must fail")
	}
}

func TestSageStateCommandsEmitJSONWithoutStartingCapture(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, command := range []string{"status", "pause", "doctor", "resume"} {
		var output bytes.Buffer
		if err := writeSageState(command, nil, &output); err != nil {
			t.Fatal(err)
		}
		var state sage.State
		if err := json.NewDecoder(&output).Decode(&state); err != nil {
			t.Fatalf("%s JSON: %v", command, err)
		}
		if state.Capture != "not_installed" || state.Ready || state.Paused != (command == "pause" || command == "doctor") {
			t.Fatalf("%s reported %+v", command, state)
		}
	}
	if err := writeSageState("status", []string{"unexpected"}, &bytes.Buffer{}); err == nil {
		t.Fatal("invalid arguments must fail")
	}
}

type fakeSageKnowledgeStore struct {
	query string
	topic string
}

func (f *fakeSageKnowledgeStore) SearchSageKnowledge(query, topic string, _ int) ([]sageknowledge.Entry, error) {
	f.query, f.topic = query, topic
	return []sageknowledge.Entry{{
		Signature: "git status", Topic: "git", Command: "git status --short",
		UseCount: 1, SuccessCount: 1, LastSeen: time.Unix(10, 0),
	}}, nil
}

func (f *fakeSageKnowledgeStore) ListSageTopics() ([]sageknowledge.Topic, error) {
	return nil, nil
}

func TestSageSearchParsesTopicAndRendersKnowledge(t *testing.T) {
	store := &fakeSageKnowledgeStore{}
	var output bytes.Buffer
	if err := writeSageSearch(store, []string{"status", "command", "--topic", "git"}, &output); err != nil {
		t.Fatal(err)
	}
	if store.query != "status command" || store.topic != "git" {
		t.Fatalf("query=%q topic=%q", store.query, store.topic)
	}
	if !bytes.Contains(output.Bytes(), []byte("## git status")) {
		t.Fatalf("output=%q", output.String())
	}
	if err := writeSageSearch(store, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("missing query must fail")
	}
}

func TestSageSearchRendersCompleteCurrentEntry(t *testing.T) {
	database, err := db.NewDatabaseAtPath(filepath.Join(t.TempDir(), "sage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	w := sageknowledge.Writer{Root: t.TempDir()}
	draft := distill.Draft{Title: "Review", Section: "Committing", Commands: []string{"git diff --staged"}, What: "Inspect staged changes", Why: "Catch subtle regressions", Example: "Before commit", Notes: "Preserve this complete note"}
	if _, err := w.Write("git", "abcdef01", draft); err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteSkipped("abcdef02"); err != nil {
		t.Fatal(err)
	}
	store := sageEntryStore{database: database, writer: w}
	var out bytes.Buffer
	if err := writeSageSearch(store, []string{"subtle", "--topic", "git"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"git.md:", "### Review", "git diff --staged", "Catch subtle regressions", "Preserve this complete note"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "Skipped action") {
		t.Fatal("skip appeared in search")
	}
	for _, args := range [][]string{{" "}, {"git", "--topic", ""}, {"git", "--topic", "git", "--topic", "other"}} {
		if err := writeSageSearch(store, args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
}
