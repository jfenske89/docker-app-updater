// Package update orchestrates a single app's refresh and classifies what
// actually happened by diffing real Docker state captured before and after,
// rather than inferring it from container uptime text.
package update

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/jfenske89/docker-app-updater/internal/config"
	"github.com/jfenske89/docker-app-updater/internal/discovery"
	"github.com/jfenske89/docker-app-updater/internal/dockercli"
	"github.com/jfenske89/docker-app-updater/internal/exec"
	"github.com/jfenske89/docker-app-updater/internal/runner"
)

// Status is the outcome classification for one app.
type Status string

const (
	StatusUpdated   Status = "updated"   // recreated on a new image
	StatusRecreated Status = "recreated" // container replaced, same image
	StatusRestarted Status = "restarted" // same container, wasn't running before
	StatusUnchanged Status = "unchanged" // nothing changed
	StatusSkipped   Status = "skipped"   // no containers found; left alone
	StatusError     Status = "error"     // a command failed
)

// severity ranks statuses so a multi-service app rolls up to its most
// significant outcome: error beats updated beats recreated beats restarted
// beats unchanged.
var severity = map[Status]int{
	StatusUnchanged: 0,
	StatusRestarted: 1,
	StatusRecreated: 2,
	StatusUpdated:   3,
	StatusError:     4,
}

// Result is the outcome of updating one app.
type Result struct {
	App      discovery.App
	Status   Status
	Duration time.Duration
	Err      error
}

// Run executes app's refresh commands and after commands, snapshotting
// Docker state before and after to classify the outcome.
func Run(ctx context.Context, app discovery.App, cfg config.Config, executor exec.Executor) Result {
	started := time.Now()

	before, snapErr := dockercli.Snapshot(ctx, executor, app.Path)
	if snapErr != nil {
		logrus.Debugf("[%s] failed to snapshot before state: %s", app.Name, snapErr.Error())
	}

	skipIfNoContainers := cfg.SkipsAppsWithoutContainers()
	if app.SkipIfNoContainers != nil {
		skipIfNoContainers = *app.SkipIfNoContainers
	}
	if skipIfNoContainers && snapErr == nil && len(before) == 0 {
		logrus.Debugf("[%s] no containers found, skipping", app.Name)
		return Result{App: app, Status: StatusSkipped, Duration: time.Since(started)}
	}

	if err := runner.RunAll(ctx, executor, app.RefreshCommands, app.Path, app.Name, cfg.CommandTimeoutDuration, cfg.DryRun); err != nil {
		return Result{App: app, Status: StatusError, Duration: time.Since(started), Err: err}
	}

	if cfg.DryRun {
		return Result{App: app, Status: StatusUnchanged, Duration: time.Since(started)}
	}

	after, snapErr := dockercli.Snapshot(ctx, executor, app.Path)
	if snapErr != nil {
		return Result{App: app, Status: StatusError, Duration: time.Since(started), Err: snapErr}
	}

	if err := runner.RunAll(ctx, executor, app.AfterCommands, app.Path, app.Name, cfg.CommandTimeoutDuration, cfg.DryRun); err != nil {
		return Result{App: app, Status: StatusError, Duration: time.Since(started), Err: err}
	}

	return Result{App: app, Status: Classify(before, after), Duration: time.Since(started)}
}

// Classify compares before/after service snapshots and rolls the diff up
// into a single app-level status.
func Classify(before, after map[string]dockercli.ServiceState) Status {
	if len(after) == 0 {
		return StatusUnchanged
	}

	status := StatusUnchanged
	for service, post := range after {
		pre, existed := before[service]

		var serviceStatus Status
		switch {
		case !existed:
			// A service with no prior snapshot (new to the project, or the
			// before-snapshot failed) can't be classified precisely; treat
			// its appearance as a recreation rather than guessing "updated".
			serviceStatus = StatusRecreated
		case pre.ContainerID != post.ContainerID && pre.ImageID != post.ImageID:
			serviceStatus = StatusUpdated
		case pre.ContainerID != post.ContainerID:
			serviceStatus = StatusRecreated
		case !pre.Running && post.Running:
			serviceStatus = StatusRestarted
		default:
			serviceStatus = StatusUnchanged
		}

		if severity[serviceStatus] > severity[status] {
			status = serviceStatus
		}
	}

	return status
}
