package main

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	config     Config
	configOnce sync.Once
)

type Config struct {
	DryRun          bool       `json:"dry_run"`
	LogLevel        string     `json:"log_level"`
	MaxThreads      int        `json:"max_threads"`
	RefreshCommands [][]string `json:"refresh_commands"`
	Apps            []App      `json:"apps"`
	AfterCommands   [][]string `json:"after_commands"`
}

type App struct {
	Name          string     `json:"name"`
	Path          string     `json:"path"`
	AfterCommands [][]string `json:"after_commands"`
}

var defaultConfig = Config{
	LogLevel:   "info",
	MaxThreads: 3,
	RefreshCommands: [][]string{
		{"docker", "compose", "pull"},
		{"docker", "compose", "up", "-d", "--remove-orphans"},
	},
}

func GetConfig() Config {
	configOnce.Do(func() { config = loadConfig() })
	return config
}

func loadConfig() Config {
	var path string
	var possiblePaths []string

	if possiblePath := os.Getenv("CONFIG_FILE"); possiblePath != "" {
		possiblePaths = append(possiblePaths, possiblePath)
	}

	if wd, err := os.Getwd(); err == nil {
		possiblePaths = append(possiblePaths, wd+"/config/config.json")
	}

	if hd, err := os.UserHomeDir(); err == nil {
		possiblePaths = append(
			possiblePaths,
			hd+"/.config/docker-app-updater/config.json",
		)
	}

	possiblePaths = append(
		possiblePaths,
		"/etc/docker-app-updater/config.json",
	)

	for _, possiblePath := range possiblePaths {
		logrus.Debugf("checking config path: %s", possiblePath)

		fileInfo, err := os.Stat(possiblePath)

		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			logrus.Warnf("failed to access config file %s: %v", possiblePath, err)
			continue
		}

		if fileInfo.IsDir() {
			logrus.Warnf("config file path %s is a directory, skipping", possiblePath)
			continue
		}

		path = possiblePath

		break
	}

	if path == "" {
		logrus.Warn("no valid config file found, using default configuration")
		config = defaultConfig
		return config
	}

	config := new(Config)
	if jsonBytes, err := os.ReadFile(path); err != nil {
		logrus.Warnf("failed to read config file: %v", err)
	} else if err := json.Unmarshal(
		jsonBytes,
		config,
	); err != nil {
		logrus.Warnf("failed to parse config: %s", err.Error())
		config = &defaultConfig
	}

	postProcessConfig(config)

	return *config
}

func postProcessConfig(cfg *Config) {
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultConfig.LogLevel
	}

	if cfg.Apps == nil {
		cfg.Apps = []App{}
	}

	if cfg.MaxThreads <= 0 {
		logrus.Warnf("invalid max_threads %d, using default (%d)", cfg.MaxThreads, defaultConfig.MaxThreads)
		cfg.MaxThreads = defaultConfig.MaxThreads
	} else if cfg.MaxThreads > 100 {
		logrus.Warnf("max_threads too high (%d), capping at 100", cfg.MaxThreads)
		cfg.MaxThreads = 100
	}

	if cfg.RefreshCommands == nil {
		cfg.RefreshCommands = defaultConfig.RefreshCommands
	}
}
