// Package runner executes user-configured command lists (refresh_commands,
// after_commands), substituting {{app.name}}/{{app.path}}/{{HOME}} and
// retrying a command whose output looks like a transient registry
// rate-limit response with exponential backoff and jitter.
package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/jfenske89/docker-app-updater/internal/exec"
)

// Run executes cmd (a command name followed by its arguments) in dir,
// substituting variables in every token. If dryRun is true, the command is
// logged but not executed. If the executor's output looks like a registry
// rate-limit response, Run retries it per retry, with exponential backoff
// and jitter between attempts.
func Run(
	ctx context.Context,
	executor exec.Executor,
	cmd []string,
	dir string,
	appName string,
	timeout time.Duration,
	dryRun bool,
	retry RetryOptions,
) (string, error) {
	if len(cmd) == 0 {
		return "", nil
	}

	command := replaceVariables(cmd[0], dir, appName)
	arguments := make([]string, 0, len(cmd)-1)
	for _, arg := range cmd[1:] {
		arguments = append(arguments, replaceVariables(arg, dir, appName))
	}

	if dryRun {
		logrus.Infof("[%s] [path=%s] dry run: %s %s", appName, dir, command, strings.Join(arguments, " "))
		return "***DRY RUN***", nil
	}

	maxAttempts := max(retry.MaxAttempts, 1)

	var output string
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		output, err = executor(runCtx, command, arguments, dir)
		cancel()

		if err == nil || attempt == maxAttempts || !isRetryable(output, err) {
			break
		}

		delay := backoffDelay(retry.BaseDelay, attempt)
		logrus.Warnf(
			"[%s] [%s %s] rate-limited, retrying in %s (attempt %d/%d)",
			appName, command, strings.Join(arguments, " "), delay, attempt, maxAttempts,
		)

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return output, fmt.Errorf(
				"failed to run %s %s: %w - retry aborted: %w",
				command, strings.Join(arguments, " "), err, ctx.Err(),
			)
		}
	}

	if err != nil {
		return output, fmt.Errorf(
			"failed to run %s %s: %w - %s",
			command,
			strings.Join(arguments, " "),
			err,
			strings.ReplaceAll(strings.ReplaceAll(output, "\n", "\\n"), "\t", "\\t"),
		)
	}

	logrus.Debugf(
		"[%s] [%s %s] [path=%s]: %s",
		appName,
		command,
		strings.Join(arguments, " "),
		dir,
		strings.ReplaceAll(strings.ReplaceAll(output, "\n", "\\n"), "\t", "\\t"),
	)

	return output, nil
}

// RunAll runs each command in cmds in order, stopping at the first error.
func RunAll(
	ctx context.Context,
	executor exec.Executor,
	cmds [][]string,
	dir string,
	appName string,
	timeout time.Duration,
	dryRun bool,
	retry RetryOptions,
) error {
	for _, cmd := range cmds {
		if _, err := Run(ctx, executor, cmd, dir, appName, timeout, dryRun, retry); err != nil {
			return err
		}
	}
	return nil
}

func replaceVariables(input string, path string, appName string) string {
	varReplacer := strings.NewReplacer(
		"{{app.name}}", appName,
		"{{app.path}}", path,
		"{{HOME}}", os.Getenv("HOME"),
	)
	return varReplacer.Replace(input)
}
