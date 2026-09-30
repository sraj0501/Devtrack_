package infra

import (
	"context"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
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
	return distill.NewBackgroundWorker(queue, client).Start(ctx)
}
