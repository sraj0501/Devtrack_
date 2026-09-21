package distill

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
)

type memoryQueue struct {
	mu        sync.Mutex
	jobs      []Job
	completed []string
	skipped   []string
	retried   []string
	changed   chan struct{}
}

func (q *memoryQueue) Claim(context.Context) (Job, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return Job{}, false, nil
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, true, nil
}

func (q *memoryQueue) Complete(_ context.Context, id string, _ Draft) error {
	q.record(&q.completed, id)
	return nil
}

func (q *memoryQueue) Skip(_ context.Context, id, _ string) error {
	q.record(&q.skipped, id)
	return nil
}

func (q *memoryQueue) Retry(_ context.Context, id string, _ error) error {
	q.record(&q.retried, id)
	return nil
}

func (q *memoryQueue) record(target *[]string, id string) {
	q.mu.Lock()
	*target = append(*target, id)
	q.mu.Unlock()
	select {
	case q.changed <- struct{}{}:
	default:
	}
}

func TestWorkerStartReturnsImmediatelyAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	queue := &memoryQueue{jobs: []Job{{ID: "one", Fact: Fact{Command: "git status"}}}, changed: make(chan struct{}, 1)}
	worker := Worker{
		Queue:     queue,
		Distiller: Distiller{Client: fakeClient{wait: true}, Timeout: time.Hour},
		IdleDelay: time.Millisecond,
	}
	started := time.Now()
	done := worker.Start(ctx)
	if time.Since(started) > 50*time.Millisecond {
		t.Fatal("Start blocked on background model work")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestWorkerPersistsCompleteSkipAndRetryOutcomes(t *testing.T) {
	cases := []struct {
		name     string
		response string
		err      error
		field    func(*memoryQueue) []string
	}{
		{
			name:     "complete",
			response: `{"worth_documenting":true,"title":"Inspect repository state","section":"Inspection","commands":["git status"],"what":"Shows changes.","why":"Checks work.","example":"git status","notes":""}`,
			field:    func(q *memoryQueue) []string { return q.completed },
		},
		{
			name: "skip", response: `{"worth_documenting":false,"skip_reason":"trivial"}`,
			field: func(q *memoryQueue) []string { return q.skipped },
		},
		{
			name: "retry", err: errors.New("offline"),
			field: func(q *memoryQueue) []string { return q.retried },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queue := &memoryQueue{jobs: []Job{{ID: tc.name, Fact: Fact{Command: "git status"}}}, changed: make(chan struct{}, 1)}
			worker := Worker{Queue: queue, Distiller: Distiller{Client: fakeClient{response: tc.response, err: tc.err}}, IdleDelay: time.Millisecond, RetryDelay: time.Millisecond}
			done := worker.Start(ctx)
			select {
			case <-queue.changed:
			case <-time.After(time.Second):
				t.Fatal("worker did not persist an outcome")
			}
			cancel()
			<-done
			queue.mu.Lock()
			got := append([]string(nil), tc.field(queue)...)
			queue.mu.Unlock()
			if len(got) != 1 || got[0] != tc.name {
				t.Fatalf("persisted=%v", got)
			}
		})
	}
}

func TestNewBackgroundWorkerUsesBoundedConfig(t *testing.T) {
	t.Setenv("DEVTRACK_SAGE_MODEL_TIMEOUT_SECS", "420")
	t.Setenv("DEVTRACK_SAGE_IDLE_POLL_MS", "125")
	t.Setenv("DEVTRACK_SAGE_RETRY_DELAY_SECS", "4")
	worker := NewBackgroundWorker(&memoryQueue{}, llmclient.Config{})
	if worker.Distiller.Timeout != 420*time.Second {
		t.Fatalf("model timeout=%s", worker.Distiller.Timeout)
	}
	if worker.IdleDelay != 125*time.Millisecond {
		t.Fatalf("idle delay=%s", worker.IdleDelay)
	}
	if worker.RetryDelay != 4*time.Second {
		t.Fatalf("retry delay=%s", worker.RetryDelay)
	}
}
