// Package runner executes user-configured command lists (refresh_commands,
// after_commands), substituting {{app.name}}/{{app.path}}/{{HOME}}.
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
// logged but not executed.
func Run(
	ctx context.Context,
	executor exec.Executor,
	cmd []string,
	dir string,
	appName string,
	timeout time.Duration,
	dryRun bool,
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

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := executor(runCtx, command, arguments, dir)
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
) error {
	for _, cmd := range cmds {
		if _, err := Run(ctx, executor, cmd, dir, appName, timeout, dryRun); err != nil {
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
