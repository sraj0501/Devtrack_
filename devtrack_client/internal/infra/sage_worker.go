package infra

import (
	"context"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
)

type pausedSageQueue struct {
	*db.SageQueue
	root string
}

func (q pausedSageQueue) Claim(ctx context.Context) (distill.Job, bool, error) {
	state, err := sage.ReadState(q.root)
	if err != nil || state.Paused {
		return distill.Job{}, false, err
	}
	return q.SageQueue.Claim(ctx)
}

func (im *IntegratedMonitor) startSageDistillation(ctx context.Context, root string, client llmclient.Config) <-chan struct{} {
	queue := pausedSageQueue{SageQueue: &db.SageQueue{
		Database:   im.database,
		Lease:      (time.Duration(config.GetSageModelTimeoutSecs()) + 30) * time.Second,
		RetryDelay: time.Duration(config.GetSageRetryDelaySecs()) * time.Second,
	}, root: root}
	modelDone := distill.NewBackgroundWorker(queue, client).Start(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { <-modelDone }()
		writer := knowledge.Writer{Root: config.GetSageKnowledgeDir(root)}
		ticker := time.NewTicker(time.Duration(config.GetSageIdlePollMS()) * time.Millisecond)
		defer ticker.Stop()
		for ctx.Err() == nil {
			state, err := sage.ReadState(root)
			if err == nil && !state.Paused {
				found, err := queue.PublishNext(ctx, writer)
				if found && err == nil {
					continue
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}
