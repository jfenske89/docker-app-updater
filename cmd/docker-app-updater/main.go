package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
)

func parseFlagsForConfigFile() string {
	flagSet := flag.NewFlagSet("config", flag.ContinueOnError)
	configFile := flagSet.String("config", "", "Path to a config file")
	_ = flagSet.Parse(os.Args[1:])

	if *configFile != "" {
		return strings.TrimSpace(*configFile)
	}

	return ""
}

func main() {
	config := GetConfig(parseFlagsForConfigFile())

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

			cmds := append(config.RefreshCommands, app.AfterCommands...)

			for _, cmd := range cmds {
				if _, err := executeCommand(cmd, app.Path, app.Name, config); err != nil {
					// don't execute anymore commands for this app
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
			"[%s] [path=%s] Dry run: %s %s",
			appName,
			path,
			command,
			strings.Join(arguments, " "),
		)

		return "***DRY RUN***", nil
	} else {
		cmd := exec.Command(command, arguments...)
		if path != "" {
			cmd.Dir = path
		}

		if output, err := cmd.CombinedOutput(); err != nil {
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
				"[%s] [%s %s] [path=%s]: %s",
				appName,
				command,
				strings.Join(arguments, " "),
				path,
				strings.ReplaceAll(strings.ReplaceAll(
					string(output), "\n", "\\n"), "\t", "\\t"),
			)

			return string(output), nil
		}
	}
}

func replaceVariables(input string, path string, appName string) string {
	varReplacer := strings.NewReplacer(
		"{{app.name}}", appName,
		"{{app.path}}", path,
		"{{HOME}}", os.Getenv("HOME"),
	)
	return varReplacer.Replace(input)
}
