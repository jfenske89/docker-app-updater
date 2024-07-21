package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
)

func main() {
	config := GetConfig()

	if level, err := logrus.ParseLevel(config.LogLevel); err != nil {
		logrus.SetLevel(logrus.InfoLevel)
		logrus.Warnf(
			"failed to parse LOG_LEVEL %s: %s",
			config.LogLevel,
			err.Error(),
		)
	} else {
		logrus.SetLevel(level)
		logrus.Debugf("log level: %s", level.String())
	}

	p := pool.New().WithMaxGoroutines(config.MaxThreads)
	logrus.Debugf("max threads: %d", config.MaxThreads)

	for i := range config.Apps {
		app := config.Apps[i]
		logrus.Debugf("[%s] updating", app.Name)

		p.Go(func() {
			started := time.Now()

			for _, cmd := range config.RefreshCommands {
				if _, err := executeCommand(cmd, app.Path, app.Name, config); err != nil {
					// Don't execute anymore commands for this app
					logrus.Errorf(
						"[%s] %s",
						app.Name,
						err.Error(),
					)
					return
				}
			}

			for _, afterCmd := range app.AfterCommands {
				if _, err := executeCommand(afterCmd, app.Path, app.Name, config); err != nil {
					// Don't execute anymore commands for this app
					logrus.Errorf(
						"[%s] %s",
						app.Name,
						err.Error(),
					)
					return
				}
			}

			if !config.DryRun {
				logrus.Infof("[%s] updated after %s", app.Name, time.Since(started))
			}
		})
	}

	p.Wait()

	if config.AfterCommands != nil {
		fmt.Println("****************************************************************")
	}

	for _, afterCmd := range config.AfterCommands {
		if output, err := executeCommand(afterCmd, "", "POST-UPDATE", config); err != nil {
			// don't execute anymore post-update commands
			logrus.Errorf(
				"[POST-UPDATE]\n$ %s\n[ERROR] %s",
				strings.Join(afterCmd, " "),
				err.Error(),
			)
			return
		} else {
			fmt.Printf(
				"[POST-UPDATE]\n$ %s\n  %s\n",
				strings.Join(afterCmd, " "),
				strings.TrimSuffix(strings.ReplaceAll(output, "\n", "\n  "), "\n"),
			)
		}
	}
}

func executeCommand(cmd []string, path string, appName string, config Config) (string, error) {
	var command string
	var arguments []string
	for _, arg := range cmd {
		if command == "" {
			command = replaceVariables(arg, path, appName)
		} else {
			arguments = append(arguments, replaceVariables(arg, path, appName))
		}
	}

	if config.DryRun {
		logrus.Infof(
			"[%s] Dry run: %s %s",
			appName,
			command,
			strings.Join(arguments, " "),
		)

		return "***DRY RUN***", nil
	} else {
		if path != "" {
			oldPath, _ := os.Getwd()
			os.Chdir(path)
			defer func() {
				os.Chdir(oldPath)
			}()
		}

		if output, err := exec.Command(
			command,
			arguments...,
		).CombinedOutput(); err != nil {
			return string(output), fmt.Errorf(
				"failed to run %s %s: %s - %s",
				command,
				strings.Join(arguments, " "),
				err.Error(),
				strings.ReplaceAll(strings.ReplaceAll(
					string(output), "\n", "\\n"), "\t", "\\t"),
			)
		} else {
			logrus.Debugf(
				"[%s] [%s %s]: %s",
				appName,
				command,
				strings.Join(arguments, " "),
				strings.ReplaceAll(strings.ReplaceAll(
					string(output), "\n", "\\n"), "\t", "\\t"),
			)

			return string(output), nil
		}
	}
}

func replaceVariables(input string, path string, appName string) string {
	output := strings.ReplaceAll(input, "{{app.name}}", appName)
	output = strings.ReplaceAll(output, "{{app.path}}", path)
	output = strings.ReplaceAll(output, "{{HOME}}", os.Getenv("HOME"))
	return output
}
