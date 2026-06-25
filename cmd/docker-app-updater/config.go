package main

import (
	"encoding/json"
	"errors"
	"os"

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
	Skip          bool       `json:"skip"`
}

var defaultConfig = Config{
	LogLevel:   "info",
	MaxThreads: 3,
	RefreshCommands: [][]string{
		{"docker", "compose", "pull"},
		{"docker", "compose", "up", "-d", "--remove-orphans"},
	},
}

func GetConfig(configFile string) Config {
	return loadConfig(configFile)
}

func loadConfig(configFile string) Config {
	var path string
	var possiblePaths []string

	if configFile != "" {
		possiblePaths = append(possiblePaths, configFile)
	} else {
		if wd, err := os.Getwd(); err == nil {
			possiblePaths = append(possiblePaths, wd+"/config/config.json")
		}

		if hd, err := os.UserHomeDir(); err == nil {
			possiblePaths = append(
				possiblePaths,
				hd+"/.config/docker-app-updater/config.json",
			)
		}

		possiblePaths = append(possiblePaths, "/etc/docker-app-updater/config.json")
	}

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
		return defaultConfig
	}

	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		logrus.Warnf("failed to read config file: %v", err)
		return defaultConfig
	}

	return parseConfig(jsonBytes)
}

func parseConfig(data []byte) Config {
	cfg := new(Config)
	if err := json.Unmarshal(data, cfg); err != nil {
		logrus.Warnf("failed to parse config: %s", err.Error())
		*cfg = defaultConfig
	}
	postProcessConfig(cfg)
	return *cfg
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
		cfg.RefreshCommands = append([][]string(nil), defaultConfig.RefreshCommands...)
	}
}
