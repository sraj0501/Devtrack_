package command

import "testing"

func signatures(raw string) []string {
	actions := Parse(raw)
	out := make([]string, len(actions))
	for i, action := range actions {
		out[i] = action.Signature
	}
	return out
}

func TestReferenceCommandSignatures(t *testing.T) {
	equal := func(left, right string) {
		t.Helper()
		a, b := signatures(left), signatures(right)
		if len(a) != len(b) || len(a) != 1 || a[0] != b[0] {
			t.Fatalf("%q => %v; %q => %v", left, a, right, b)
		}
	}
	different := func(left, right string) {
		t.Helper()
		a, b := signatures(left), signatures(right)
		if len(a) != 1 || len(b) != 1 || a[0] == b[0] {
			t.Fatalf("%q => %v; %q => %v", left, a, right, b)
		}
	}

	equal(`git commit -m "fix a"`, `git commit -m "fix b"`)
	equal("npm install lodash", "npm install express")
	equal("brew services restart postgresql@16", "brew services restart redis")
	equal("systemctl status sshd", "systemctl status httpd")
	equal("helm upgrade api ./chart", "helm upgrade web ./chart")
	equal("kubectl logs pod-abc", "kubectl logs pod-xyz")
	equal("dnf install ripgrep", "dnf install jq")
	different("systemctl status sshd", "systemctl restart sshd")
	different("helm repo add x https://y", "helm repo update")
	different("kubectl get pods", "kubectl get svc")
	different("az boards work-item show", "az boards query")
	different("npm run test", "npm run build")
	different("git --no-pager diff", "git --no-pager log")
	different("git stash push", "git stash pop")
}

func TestReferencePipelinesYieldOneSignaturePerSegment(t *testing.T) {
	if got := signatures("cat f.json | python3 -m json.tool"); len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got := signatures("git fetch upstream && git merge upstream/staging"); len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestReferenceQuotedSeparatorsDoNotSplit(t *testing.T) {
	if got := signatures(`git commit -m "a; b && c"`); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

func TestReferenceTrivialCommandsAreIgnored(t *testing.T) {
	for _, raw := range []string{"echo hello", "cd /tmp", "pwd", "ls"} {
		if got := signatures(raw); len(got) != 0 {
			t.Fatalf("%q => %v", raw, got)
		}
	}
}

func TestIdentityIgnoresFlagsButKeepsNestedSubcommands(t *testing.T) {
	if Identity("git status --short") != Identity("git status --porcelain=v2") {
		t.Fatal("flag variants did not match")
	}
	if Identity("npm run test") == Identity("npm run build") {
		t.Fatal("nested subcommands collapsed")
	}
}
