package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chaos-synthesis/mattermost-plugin-retention/server/channels"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/config"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/wiggin77/merror"
)

func (p *Plugin) runJob() {
	// Include job logic here
	p.API.LogInfo("Retention Job is currently running")

	exitSignal := make(chan struct{})
	ctx, canceller := context.WithCancel(context.Background())

	runner := &runInstance{
		canceller:  canceller,
		exitSignal: exitSignal,
	}

	var oldRunner *runInstance
	p.backgroundJobHelper.mux.Lock()
	oldRunner = p.backgroundJobHelper.runner
	p.backgroundJobHelper.runner = runner
	p.backgroundJobHelper.mux.Unlock()

	defer func() {
		close(exitSignal)
		p.backgroundJobHelper.mux.Lock()
		p.backgroundJobHelper.runner = nil
		p.backgroundJobHelper.mux.Unlock()
	}()

	if oldRunner != nil {
		p.API.LogError("Multiple Posts Retention jobs scheduled concurrently; there can be only one")
		return
	}

	opts := channels.ArchiverOpts{
		StalePostOpts: store.StalePostOpts{
			AgeInSeconds: p.getConfiguration().AgeInSeconds,
		},
		BatchSize: p.getConfiguration().BatchSize,
	}

	results, err := channels.RemoveStalePostsWithApi(ctx, p.sqlStore, p.client, opts, p.API)
	if err != nil {
		p.API.LogError("Error running Posts Retention job", err)
		return
	}

	p.API.LogInfo("Posts Retention job", "posts_deleted", results.PostsDeleted, "status", results.ExitReason, "duration", results.Duration.String())
}

type PostRetentionJobHelper struct {
	mux    sync.Mutex
	runner *runInstance
	plugin *Plugin
}

func (j *PostRetentionJobHelper) OnConfigurationChange() error {
	if j.plugin == nil {
		return nil
	}

	// stop existing job (if any)
	if err := j.Stop(time.Second * 15); err != nil {
		j.plugin.API.LogError("Error stopping Posts Retention job for config change", "err", err)
	}

	return j.Start()
}

func (j *PostRetentionJobHelper) Start() error {
	j.mux.Lock()
	defer j.mux.Unlock()

	p := j.plugin

	settings, err := p.getConfiguration().GetPostRetentionJobSettings()
	if err != nil {
		return err
	}

	if settings.EnableRetentionPolicy {
		job, err := cluster.Schedule(p.API, "posts_retention_policy_background_job", j.nextWaitInterval, p.runJob)
		if err != nil {
			return fmt.Errorf("cannot start Posts Retention: %w", err)
		}
		p.backgroundJob = job

		j.plugin.API.LogDebug("Posts Retention started", "dow", settings.DayOfWeek)
	}

	return nil
}

func (j *PostRetentionJobHelper) Stop(timeout time.Duration) error {
	var job *cluster.Job
	var runner *runInstance

	j.mux.Lock()
	job = j.plugin.backgroundJob
	runner = j.runner
	j.plugin.backgroundJob = nil
	j.runner = nil
	j.mux.Unlock()

	mErr := merror.New()

	if job != nil {
		if err := job.Close(); err != nil {
			mErr.Append(fmt.Errorf("error closing job: %w", err))
		}
	}

	if runner != nil {
		if err := runner.stop(timeout); err != nil {
			mErr.Append(fmt.Errorf("error stopping job runner: %w", err))
		}
	}

	j.plugin.API.LogDebug("Posts Retention stopped", "err", mErr.ErrorOrNil())

	return mErr.ErrorOrNil()
}

func (j *PostRetentionJobHelper) nextWaitInterval(now time.Time, metadata cluster.JobMetadata) time.Duration {
	lastFinished := metadata.LastFinished
	if lastFinished.IsZero() {
		lastFinished = now
	}

	settings, _ := j.plugin.getConfiguration().GetPostRetentionJobSettings()

	next := settings.Frequency.CalcNext(lastFinished, settings.DayOfWeek, settings.TimeOfDay)
	delta := next.Sub(now)
	// Debug
	//delta = (15 * time.Second) - now.Sub(metadata.LastFinished)

	j.plugin.API.LogDebug("Posts Retention next run scheduled", "last", lastFinished.Format(config.FullLayout), "next", next.Format(config.FullLayout), "wait", delta.String())

	return delta
}

type runInstance struct {
	canceller  func()        // called to stop a currently executing run
	exitSignal chan struct{} // closed when the currently executing run has exited
}

func (r *runInstance) stop(timeout time.Duration) error {
	// cancel the run
	r.canceller()

	// wait for it to exit
	select {
	case <-r.exitSignal:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("waiting on job to stop timed out after %s", timeout.String())
	}
}
