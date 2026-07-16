package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"

	"github.com/jfenske89/docker-app-updater/internal/config"
	"github.com/jfenske89/docker-app-updater/internal/discovery"
	"github.com/jfenske89/docker-app-updater/internal/exec"
	"github.com/jfenske89/docker-app-updater/internal/notify/gotify"
	"github.com/jfenske89/docker-app-updater/internal/report"
	"github.com/jfenske89/docker-app-updater/internal/runner"
	"github.com/jfenske89/docker-app-updater/internal/update"
)

type flags struct {
	configPath     string
	profile        string
	dryRun         bool
	discoverOnly   bool
	noNotify       bool
	logLevel       string
	maxThreads     int
	commandTimeout string
}

func parseFlags() flags {
	configPath := flag.String("config", "", "Path to a config file")
	profile := flag.String("profile", "", "Only run apps with no profiles set, plus apps tagged with this profile")
	dryRun := flag.Bool("dry-run", false, "Log commands without executing them (overrides config)")
	discoverOnly := flag.Bool("discover-only", false, "Print the resolved app list and exit, without updating anything")
	noNotify := flag.Bool("no-notify", false, "Skip sending the Gotify notification, printing the report instead")
	logLevel := flag.String("log-level", "", "A logrus level, e.g. debug/info/warn (overrides config)")
	maxThreads := flag.Int("max-threads", 0, "Apps processed concurrently, capped at 100 (overrides config)")
	commandTimeout := flag.String("command-timeout", "", "A Go duration each command may run before being killed, e.g. 15m (overrides config)")
	flag.Parse()

	return flags{
		configPath:     strings.TrimSpace(*configPath),
		profile:        strings.TrimSpace(*profile),
		dryRun:         *dryRun,
		discoverOnly:   *discoverOnly,
		noNotify:       *noNotify,
		logLevel:       strings.TrimSpace(*logLevel),
		maxThreads:     *maxThreads,
		commandTimeout: strings.TrimSpace(*commandTimeout),
	}
}

func main() {
	os.Exit(run())
}

func run() int {
	f := parseFlags()

	cfg, err := config.Load(f.configPath)
	if err != nil {
		logrus.Errorf("failed to load config: %s", err.Error())
		return 1
	}

	if f.logLevel != "" {
		cfg.LogLevel = f.logLevel
	}
	if level, err := logrus.ParseLevel(cfg.LogLevel); err != nil {
		logrus.SetLevel(logrus.InfoLevel)
		logrus.Warnf("failed to parse log_level %s: %s", cfg.LogLevel, err.Error())
	} else {
		logrus.SetLevel(level)
	}

	if f.dryRun {
		cfg.DryRun = true
	}

	if f.maxThreads > 0 {
		cfg.MaxThreads = config.NormalizeMaxThreads(f.maxThreads)
	}

	if f.commandTimeout != "" {
		cfg.CommandTimeout, cfg.CommandTimeoutDuration = config.NormalizeCommandTimeout(f.commandTimeout)
	}

	apps, err := discovery.Discover(cfg)
	if err != nil {
		logrus.Errorf("failed to discover apps: %s", err.Error())
		return 1
	}
	apps = discovery.FilterByProfile(apps, f.profile)

	if f.discoverOnly {
		for _, app := range apps {
			fmt.Printf("%s\t%s\n", app.Name, app.Path)
		}
		return 0
	}

	logrus.Infof("updating %d app(s) [profile=%q max_threads=%d dry_run=%t]", len(apps), f.profile, cfg.MaxThreads, cfg.DryRun)

	ctx := context.Background()
	p := pool.NewWithResults[update.Result]().WithMaxGoroutines(cfg.MaxThreads)
	for i := range apps {
		app := apps[i]
		p.Go(func() update.Result {
			logrus.Debugf("[%s] updating", app.Name)
			result := update.Run(ctx, app, cfg, exec.OS)
			if result.Err != nil {
				logrus.Errorf("[%s] %s", app.Name, result.Err.Error())
			} else if result.Status != update.StatusUnchanged {
				logrus.Infof("[%s] %s after %s", app.Name, result.Status, result.Duration)
			}
			return result
		})
	}
	results := p.Wait()

	hadFailure := false
	for _, r := range results {
		if r.Status == update.StatusError {
			hadFailure = true
		}
	}

	if len(cfg.AfterCommands) > 0 {
		if err := runner.RunAll(ctx, exec.OS, cfg.AfterCommands, "", "GLOBAL", cfg.CommandTimeoutDuration, cfg.DryRun); err != nil {
			logrus.Errorf("[after_commands] %s", err.Error())
			hadFailure = true
		}
	}

	if err := notify(ctx, cfg, results, f.noNotify); err != nil {
		logrus.Errorf("failed to send notification: %s", err.Error())
		hadFailure = true
	}

	if hadFailure {
		return 1
	}
	return 0
}

func notify(ctx context.Context, cfg config.Config, results []update.Result, noNotify bool) error {
	message := report.Build(cfg.Gotify.Label, results)
	if message == "" {
		logrus.Info("nothing to report, no notification sent")
		return nil
	}

	if cfg.DryRun {
		logrus.Infof("dry run: would send notification:\n%s", message)
		return nil
	}

	if noNotify {
		logrus.Info("notifications disabled, skipping notification:")
		fmt.Println(message)
		return nil
	}

	if cfg.Gotify.URL == "" || cfg.Gotify.Token == "" {
		logrus.Warn("gotify.url/token not configured, skipping notification:")
		fmt.Println(message)
		return nil
	}

	client := gotify.NewClient(cfg.Gotify.URL, cfg.Gotify.Token)
	if err := client.Send(ctx, message, cfg.Gotify.Priority); err != nil {
		return err
	}

	logrus.Info("notification sent")
	return nil
}
