package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
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

var staticConfig *Config

func GetConfig() Config {
	if staticConfig == nil {
		var path string
		var possiblePaths []string

		if possiblePath := os.Getenv("CONFIG_FILE"); possiblePath != "" {
			possiblePaths = append(possiblePaths, possiblePath)
		}

		if wd, err := os.Getwd(); err == nil {
			possiblePaths = append(
				possiblePaths,
				strings.TrimSuffix(wd, "/")+"/config/config.json",
			)
		}

		if hd, err := os.UserHomeDir(); err == nil {
			possiblePaths = append(
				possiblePaths,
				strings.TrimSuffix(hd, "/")+"/.config/docker-app-updater/config.json",
			)
		}

		possiblePaths = append(
			possiblePaths,
			"/etc/docker-app-updater/config.json",
		)

		for _, possiblePath := range possiblePaths {
			if _, err := os.Stat(possiblePath); errors.Is(err, os.ErrNotExist) {
				continue
			} else {
				path = possiblePath
				break
			}
		}

		staticConfig = new(Config)
		if jsonBytes, err := os.ReadFile(path); err != nil {
			logrus.Warnf("failed to read config file: %s", err.Error())
		} else if err := json.Unmarshal(
			jsonBytes,
			staticConfig,
		); err != nil {
			logrus.Warnf("failed to parse config: %s", err.Error())
			staticConfig = &defaultConfig
		}

		if staticConfig.LogLevel == "" {
			staticConfig.LogLevel = defaultConfig.LogLevel
		}

		if staticConfig.MaxThreads == 0 {
			staticConfig.MaxThreads = defaultConfig.MaxThreads
		}

		if staticConfig.RefreshCommands == nil {
			staticConfig.RefreshCommands = defaultConfig.RefreshCommands
		}
	}

	return *staticConfig
}
