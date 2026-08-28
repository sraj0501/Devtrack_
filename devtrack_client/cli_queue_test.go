package main

import "testing"

func TestQueueListShowAllHandlesBareQueue(t *testing.T) {
	if queueListShowAll([]string{"devtrack", "queue"}) {
		t.Fatal("bare queue command unexpectedly enabled --all")
	}
}

func TestQueueListShowAllRecognizesFlag(t *testing.T) {
	for _, args := range [][]string{
		{"devtrack", "queue", "--all"},
		{"devtrack", "queue", "list", "--all"},
	} {
		if !queueListShowAll(args) {
			t.Fatalf("queue args %v did not enable --all", args)
		}
	}
}
