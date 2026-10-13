package infra

import (
	"context"
	"log"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/llmclient"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/distill"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/knowledge/gitcommit"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/sage/routing"
)

type pausedSageQueue struct {
	*db.SageQueue
	root, knowledgeRoot string
}

func (q pausedSageQueue) Claim(ctx context.Context) (distill.Job, bool, error) {
	state, err := sage.ReadState(q.root)
	if err != nil || state.Paused {
		return distill.Job{}, false, err
	}
	recorder := &gitcommit.Recorder{}
	writer := knowledge.Writer{Root: q.knowledgeRoot, Recorder: recorder}
	if reused, err := q.ReuseDocumented(ctx, writer); err != nil {
		return distill.Job{}, false, err
	} else if reused {
		_, commitErr := (gitcommit.Committer{KnowledgeRoot: q.knowledgeRoot,
			Enabled: config.GetSageAutoCommit()}).Commit(ctx, recorder.Changes())
		return distill.Job{}, false, commitErr
	}
	return q.SageQueue.Claim(ctx)
}

func (im *IntegratedMonitor) startSageDistillation(ctx context.Context, root string, client llmclient.Config) <-chan struct{} {
	knowledgeRoot := config.GetSageKnowledgeDir(root)
	queue := pausedSageQueue{SageQueue: &db.SageQueue{
		Database:   im.database,
		Lease:      (time.Duration(config.GetSageModelTimeoutSecs()) + 30) * time.Second,
		RetryDelay: time.Duration(config.GetSageRetryDelaySecs()) * time.Second,
	}, root: root, knowledgeRoot: knowledgeRoot}
	worker := distill.NewBackgroundWorker(queue, client)
	worker.Prepare = (routing.Preparer{
		Writer:  knowledge.Writer{Root: knowledgeRoot},
		Client:  client,
		Timeout: time.Duration(config.GetSageModelTimeoutSecs()) * time.Second,
	}).Prepare
	modelDone := worker.Start(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { <-modelDone }()
		ticker := time.NewTicker(time.Duration(config.GetSageIdlePollMS()) * time.Millisecond)
		defer ticker.Stop()
		for ctx.Err() == nil {
			state, err := sage.ReadState(root)
			if err == nil && !state.Paused {
				recorder := &gitcommit.Recorder{}
				writer := knowledge.Writer{Root: knowledgeRoot, Recorder: recorder}
				found, err := queue.PublishNext(ctx, writer)
				if found && err == nil {
					committed, commitErr := (gitcommit.Committer{
						KnowledgeRoot: knowledgeRoot,
						Enabled:       config.GetSageAutoCommit(),
					}).Commit(ctx, recorder.Changes())
					if commitErr != nil {
						log.Printf("Sage knowledge commit skipped: %v", commitErr)
					} else if committed {
						log.Printf("Sage committed local knowledge changes")
					}
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
