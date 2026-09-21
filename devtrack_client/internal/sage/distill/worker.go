package distill

import (
	"context"
	"errors"
	"time"

	devconfig "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
)

// Job is a durable queue claim handed to the background distillation worker.
type Job struct {
	ID   string
	Fact Fact
}

// Queue owns durable claim, retry, and completion state. Implementations must
// recover abandoned claims after restart; the worker never treats a model
// failure as a skip verdict.
type Queue interface {
	Claim(context.Context) (Job, bool, error)
	Complete(context.Context, string, Draft) error
	Skip(context.Context, string, string) error
	Retry(context.Context, string, error) error
}

// Worker runs only under daemon ownership. Start returns immediately and all
// model work occurs on its goroutine, never on capture or user command paths.
type Worker struct {
	Queue      Queue
	Distiller  Distiller
	IdleDelay  time.Duration
	RetryDelay time.Duration
}

// NewBackgroundWorker applies centrally configured, bounded timing defaults.
// It constructs the worker but does not run it; daemon startup owns Start.
func NewBackgroundWorker(queue Queue, client llmclient.Config) Worker {
	return Worker{
		Queue: queue,
		Distiller: Distiller{
			Client:  client,
			Timeout: time.Duration(devconfig.GetSageModelTimeoutSecs()) * time.Second,
		},
		IdleDelay:  time.Duration(devconfig.GetSageIdlePollMS()) * time.Millisecond,
		RetryDelay: time.Duration(devconfig.GetSageRetryDelaySecs()) * time.Second,
	}
}

func (w Worker) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.run(ctx)
	}()
	return done
}

func (w Worker) run(ctx context.Context) {
	if w.Queue == nil {
		return
	}
	delay := w.IdleDelay
	if delay <= 0 {
		delay = time.Second
	}
	for ctx.Err() == nil {
		job, found, err := w.Queue.Claim(ctx)
		if err != nil {
			if !wait(ctx, delay) {
				return
			}
			continue
		}
		if !found {
			if !wait(ctx, delay) {
				return
			}
			continue
		}

		outcome, err := w.Distiller.Distill(ctx, job.Fact)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			_ = w.Queue.Retry(ctx, job.ID, err)
			retryDelay := w.RetryDelay
			if retryDelay <= 0 {
				retryDelay = 2 * time.Second
			}
			if !wait(ctx, retryDelay) {
				return
			}
			continue
		}
		if outcome.Skipped {
			_ = w.Queue.Skip(ctx, job.ID, outcome.SkipReason)
			continue
		}
		if outcome.Draft == nil {
			_ = w.Queue.Retry(ctx, job.ID, errors.New("sage distiller returned no draft or skip verdict"))
			continue
		}
		_ = w.Queue.Complete(ctx, job.ID, *outcome.Draft)
	}
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
