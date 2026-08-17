package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/notify"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/onboarding"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

type firstRunVoiceClient interface {
	Ping() bool
	VoiceSeed(trigger.VoiceSeedRequest) (*trigger.VoiceSeedResponse, error)
	VoiceProfileGenerate([]string) (string, int, error)
	VoiceStatus() (*trigger.VoiceStatusResponse, error)
}

// FirstRunResult is exported through package daemon so the CLI can surface the
// durable first-run result without depending on the storage package directly.
type FirstRunResult = onboarding.Result

func ReadFirstRunResult() (*FirstRunResult, error) { return onboarding.ReadResult() }

// startFirstRunWow starts the one-time voice bootstrap. It deliberately runs
// after the Go-native daemon is usable and never blocks daemon startup.
func (d *Daemon) startFirstRunWow() {
	if config.IsExternalServer() {
		log.Println("First-run voice mining skipped in external mode; learning data remains local")
		return
	}
	if result, err := onboarding.ReadResult(); err == nil {
		log.Printf("First-run voice profile already ready from %d commits", result.CommitCount)
		return
	}
	go d.runFirstRunWow()
}

func (d *Daemon) runFirstRunWow() {
	retry := time.Duration(config.GetHealthCheckIntervalSecs()) * time.Second
	ticker := time.NewTicker(retry)
	defer ticker.Stop()

	for {
		client := trigger.NewHTTPTriggerClient()
		if client.Ping() {
			result, err := buildFirstRunProfile(client, configuredVoiceRepoPaths(), config.GetVoiceSeedMonths())
			if err != nil {
				log.Printf("First-run voice profile deferred: %v", err)
				return
			}
			if err := onboarding.WriteResult(result); err != nil {
				log.Printf("First-run voice profile ready but result marker failed: %v", err)
				return
			}
			message := fmt.Sprintf("Profile built from %d commits. Try: devtrack work report", result.CommitCount)
			log.Printf("✓ %s", message)
			if err := (notify.OS{}).Send("DevTrack voice profile ready", message, ""); err != nil {
				log.Printf("First-run desktop notification unavailable: %v", err)
			}
			return
		}

		// A user can start the Go daemon while TASK-118's optional checkout is
		// still running. Once the managed server files arrive, start the server
		// without requiring a daemon restart, then let the next loop iteration
		// perform voice setup.
		d.startManagedServerWhenInstalled()

		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Daemon) startManagedServerWhenInstalled() {
	if config.IsExternalServer() {
		return
	}
	d.webhookMu.Lock()
	alreadyStarted := d.webhookServer != nil
	d.webhookMu.Unlock()
	if alreadyStarted || !managedServerFilesAvailable() {
		return
	}
	if err := d.startWebhookServer(); err != nil {
		log.Printf("Managed AI server still unavailable during first-run setup: %v", err)
		return
	}
	d.webhookMu.Lock()
	cmd := d.webhookServer
	d.webhookMu.Unlock()
	if d.healthMonitor != nil && cmd != nil && cmd.Process != nil {
		d.healthMonitor.SetWebhookPID(cmd.Process.Pid)
	}
}

func managedServerFilesAvailable() bool {
	projectRoot := config.GetProjectRootOptional()
	if projectRoot == "" {
		home, err := config.DevtrackDataHome()
		if err != nil {
			return false
		}
		projectRoot = filepath.Join(home, "server", "devtrack_server")
	}
	_, err := os.Stat(filepath.Join(projectRoot, "backend"))
	return err == nil
}

func configuredVoiceRepoPaths() []string {
	workspaces, err := config.LoadWorkspacesConfig()
	if err != nil || workspaces == nil {
		return nil
	}
	paths := make([]string, 0, len(workspaces.Workspaces))
	seen := make(map[string]struct{})
	for _, workspace := range workspaces.GetEnabledWorkspaces() {
		path := filepath.Clean(config.ExpandWorkspacePath(workspace.Path))
		if path == "." || path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func buildFirstRunProfile(client firstRunVoiceClient, repoPaths []string, sinceMonths int) (onboarding.Result, error) {
	if status, err := client.VoiceStatus(); err == nil && status.ProfileExists {
		return onboarding.Result{
			CommitCount: status.BySource["git_history"],
			WordCount:   status.ProfileWordCount,
		}, nil
	}
	if len(repoPaths) == 0 {
		return onboarding.Result{}, fmt.Errorf("no enabled workspace is available for local voice mining")
	}

	embedded := 0
	for _, repoPath := range repoPaths {
		response, err := client.VoiceSeed(trigger.VoiceSeedRequest{
			RepoPath: repoPath, SinceMonths: sinceMonths, Force: false,
		})
		if err != nil {
			return onboarding.Result{}, fmt.Errorf("seed %s: %w", repoPath, err)
		}
		if response.Error != "" {
			return onboarding.Result{}, fmt.Errorf("seed %s: %s", repoPath, response.Error)
		}
		embedded += response.Embedded
	}

	profilePath, wordCount, err := client.VoiceProfileGenerate(repoPaths)
	if err != nil {
		return onboarding.Result{}, err
	}
	commitCount := embedded
	if status, statusErr := client.VoiceStatus(); statusErr == nil {
		if count := status.BySource["git_history"]; count > 0 {
			commitCount = count
		}
	}
	return onboarding.Result{
		CommitCount: commitCount,
		WordCount:   wordCount,
		ProfilePath: profilePath,
	}, nil
}
