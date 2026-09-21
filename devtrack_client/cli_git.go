package main

import (
	"fmt"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/gitcmd"
)

// gitCommitHooks keeps only the explicit deferred-enhancement outbox. Commit
// completion itself has no DevTrack callbacks: the daemon observes the finished
// commit asynchronously and owns ticket inference plus pending-action staging.
// This is intentionally true even for `devtrack git commit`; an explicit AI
// commit helper must not turn into a ticket, time, PM-post, or push questionnaire.
func gitCommitHooks() *gitcmd.CommitHooks {
	return &gitcmd.CommitHooks{QueueForLater: gitQueueForLater}
}

// gitQueueForLater stores the staged change for later AI enhancement in the
// deferred_commits outbox (offline-first hold-diff model). The work is not
// committed; `devtrack commits review` applies it once enhanced.
func gitQueueForLater(repoPath, message, branch, diffPatch string, files []string) (bool, error) {
	database := openDBQuiet()
	if database == nil {
		return false, fmt.Errorf("database unavailable")
	}
	defer database.Close()

	mgr := NewDeferredCommitManager(database)
	if _, err := mgr.QueueCommit(message, diffPatch, branch, repoPath, files); err != nil {
		return false, err
	}
	return true, nil
}

// openDBQuiet opens the database for an explicit CLI operation. Database
// initialization itself is silent; errors are returned to the invoking command.
func openDBQuiet() *db.Database {
	database, err := db.NewDatabase()
	if err != nil {
		return nil
	}
	return database
}
