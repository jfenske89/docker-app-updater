// Package config loads and validates the YAML configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Discovery controls where compose projects are found automatically.
type Discovery struct {
	// Roots are directories scanned one level deep for compose files, e.g.
	// a project at Roots[i]/<name>/docker-compose.yml is discovered as
	// app <name> with path Roots[i]/<name>.
	Roots []string `yaml:"roots"`

	// ComposeFilenames are the filenames that mark a subdirectory as a
	// compose project.
	ComposeFilenames []string `yaml:"compose_filenames"`
}

// Override customizes or opts a single discovered (or explicit) app into a
// subset of profiles. Keyed by app name in Config.Overrides.
type Override struct {
	RefreshCommands [][]string `yaml:"refresh_commands"`
	AfterCommands   [][]string `yaml:"after_commands"`

	// Profiles is a freeform list of tags. An app with no Profiles set is
	// always included, regardless of which --profile (if any) is passed.
	// An app with Profiles set is included only when --profile matches one
	// of them. There is no built-in profile vocabulary.
	Profiles []string `yaml:"profiles"`

	// SkipIfNoContainers overrides Config.SkipIfNoContainers for this one
	// app. Nil means "inherit the global default".
	SkipIfNoContainers *bool `yaml:"skip_if_no_containers"`
}

// App is an explicit app entry, for compose projects outside the
// discovery roots. Everything discovery finds automatically is expressed
// the same way internally, but doesn't need to be written out by hand.
type App struct {
	Name            string     `yaml:"name"`
	Path            string     `yaml:"path"`
	RefreshCommands [][]string `yaml:"refresh_commands"`
	AfterCommands   [][]string `yaml:"after_commands"`
	Profiles        []string   `yaml:"profiles"`

	// SkipIfNoContainers overrides Config.SkipIfNoContainers for this one
	// app. Nil means "inherit the global default".
	SkipIfNoContainers *bool `yaml:"skip_if_no_containers"`
}

// Gotify holds the credentials used to push a status report notification.
type Gotify struct {
	// URL and Token are literal values for the MVP. A future version may
	// accept a provider URI (e.g. "op://vault/item/field") here instead of
	// only a plain string.
	URL   string `yaml:"url"`
	Token string `yaml:"token"`

	// Priority is Gotify's message priority (0-10). Zero is Gotify's
	// normal/default priority, so no config value is treated specially.
	Priority int `yaml:"priority"`

	// Label, if set, is prepended to the message as a bold header so
	// reports from different hosts can be told apart. Empty by default.
	Label string `yaml:"label"`
}

type Config struct {
	DryRun     bool   `yaml:"dry_run"`
	LogLevel   string `yaml:"log_level"`
	MaxThreads int    `yaml:"max_threads"`

	Discovery Discovery           `yaml:"discovery"`
	Excludes  []string            `yaml:"excludes"`
	Overrides map[string]Override `yaml:"overrides"`
	Apps      []App               `yaml:"apps"`

	RefreshCommands [][]string `yaml:"refresh_commands"`
	AfterCommands   [][]string `yaml:"after_commands"`

	Gotify Gotify `yaml:"gotify"`

	CommandTimeout         string        `yaml:"command_timeout"`
	CommandTimeoutDuration time.Duration `yaml:"-"`

	// SkipIfNoContainers controls whether an app with zero containers
	// found (nothing running or ever created for its compose project) is
	// left alone rather than started. Defaults to true: discovery
	// shouldn't resurrect a project you intentionally took down with
	// "docker compose down"; only apps that are already present get
	// refreshed. Override per app via Overrides[name].SkipIfNoContainers
	// or App.SkipIfNoContainers.
	SkipIfNoContainers *bool `yaml:"skip_if_no_containers"`
}

// SkipsAppsWithoutContainers reports the effective global default: true
// unless explicitly set to false in config.
func (cfg Config) SkipsAppsWithoutContainers() bool {
	if cfg.SkipIfNoContainers == nil {
		return true
	}
	return *cfg.SkipIfNoContainers
}

const defaultCommandTimeout = "15m"

var defaultCommandTimeoutDuration = mustParseDuration(defaultCommandTimeout)

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

var defaultComposeFilenames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

var defaultRefreshCommands = [][]string{
	{"docker", "compose", "pull"},
	{"docker", "compose", "up", "-d", "--remove-orphans"},
}

const defaultMaxThreads = 3

// Load reads and validates a config file from path. If path is empty, it
// checks the same well-known locations docker-app-updater used.
func Load(path string) (Config, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(resolved) //nolint:gosec // G304: resolved is a config path from --config or a well-known default, not untrusted input
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file %s: %w", resolved, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file %s: %w", resolved, err)
	}

	postProcess(&cfg)
	return cfg, nil
}

func resolvePath(path string) (string, error) {
	var candidates []string

	if path != "" {
		candidates = append(candidates, path)
	} else {
		if wd, err := os.Getwd(); err == nil {
			candidates = append(candidates, wd+"/config/config.yaml")
		}
		if hd, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, hd+"/.config/docker-app-updater/config.yaml")
		}
		candidates = append(candidates, "/etc/docker-app-updater/config.yaml")
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() {
			logrus.Warnf("config path %s is a directory, skipping", candidate)
			continue
		}
		return candidate, nil
	}

	return "", fmt.Errorf("no config file found (checked: %v)", candidates)
}

func postProcess(cfg *Config) {
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	cfg.MaxThreads = NormalizeMaxThreads(cfg.MaxThreads)

	if len(cfg.RefreshCommands) == 0 {
		cfg.RefreshCommands = append([][]string(nil), defaultRefreshCommands...)
	}

	if len(cfg.Discovery.ComposeFilenames) == 0 {
		cfg.Discovery.ComposeFilenames = append([]string(nil), defaultComposeFilenames...)
	}

	cfg.CommandTimeout, cfg.CommandTimeoutDuration = NormalizeCommandTimeout(cfg.CommandTimeout)
}

// NormalizeMaxThreads applies the same default/cap rules used when loading
// max_threads from a config file. Exported so CLI overrides (--max-threads)
// go through identical validation.
func NormalizeMaxThreads(n int) int {
	if n <= 0 {
		return defaultMaxThreads
	}
	if n > 100 {
		logrus.Warnf("max_threads too high (%d), capping at 100", n)
		return 100
	}
	return n
}

// NormalizeCommandTimeout applies the same default/validation rules used
// when loading command_timeout from a config file. Exported so CLI
// overrides (--command-timeout) go through identical validation.
func NormalizeCommandTimeout(s string) (string, time.Duration) {
	if s == "" {
		return defaultCommandTimeout, defaultCommandTimeoutDuration
	}
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return s, d
	}
	logrus.Warnf("invalid command_timeout %q, using default (%s)", s, defaultCommandTimeout)
	return defaultCommandTimeout, defaultCommandTimeoutDuration
}
