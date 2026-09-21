package main

import "testing"

func TestGitCommitHooksHaveNoInteractiveCommitCallbacks(t *testing.T) {
	hooks := gitCommitHooks()
	if hooks == nil {
		t.Fatal("gitCommitHooks() returned nil")
	}
	if hooks.BeforeCommit != nil {
		t.Fatal("BeforeCommit must be nil: commit flow cannot show a ticket picker")
	}
	if hooks.AfterCommit != nil {
		t.Fatal("AfterCommit must be nil: commit completion cannot ask for time, PM posting, or push")
	}
	if hooks.QueueForLater == nil {
		t.Fatal("QueueForLater must remain available for explicitly requested deferred enhancement")
	}
}
