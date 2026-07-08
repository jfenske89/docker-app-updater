package main

import (
	"testing"
	"time"
)

func TestParseConfig_ValidFull(t *testing.T) {
	data := []byte(`{
		"dry_run": true,
		"log_level": "debug",
		"max_threads": 5,
		"refresh_commands": [["docker", "pull"]],
		"apps": [{"name": "myapp", "path": "/app", "after_commands": [["echo", "done"]]}],
		"after_commands": [["echo", "finished"]]
	}`)

	cfg := parseConfig(data)

	if !cfg.DryRun {
		t.Errorf("expected DryRun=true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel=debug, got %s", cfg.LogLevel)
	}
	if cfg.MaxThreads != 5 {
		t.Errorf("expected MaxThreads=5, got %d", cfg.MaxThreads)
	}
	if len(cfg.RefreshCommands) != 1 || cfg.RefreshCommands[0][0] != "docker" {
		t.Errorf("unexpected RefreshCommands: %v", cfg.RefreshCommands)
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "myapp" || cfg.Apps[0].Path != "/app" {
		t.Errorf("unexpected Apps: %v", cfg.Apps)
	}
	if len(cfg.AfterCommands) != 1 || cfg.AfterCommands[0][0] != "echo" {
		t.Errorf("unexpected AfterCommands: %v", cfg.AfterCommands)
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	cfg := parseConfig([]byte(`not json`))

	if cfg.LogLevel != defaultConfig.LogLevel {
		t.Errorf("expected default LogLevel %q, got %q", defaultConfig.LogLevel, cfg.LogLevel)
	}
	if cfg.MaxThreads != defaultConfig.MaxThreads {
		t.Errorf("expected default MaxThreads %d, got %d", defaultConfig.MaxThreads, cfg.MaxThreads)
	}
	if len(cfg.RefreshCommands) != len(defaultConfig.RefreshCommands) {
		t.Errorf("expected default RefreshCommands, got %v", cfg.RefreshCommands)
	}
}

func TestParseConfig_EmptyObject(t *testing.T) {
	cfg := parseConfig([]byte(`{}`))

	if cfg.LogLevel != defaultConfig.LogLevel {
		t.Errorf("expected default LogLevel %q, got %q", defaultConfig.LogLevel, cfg.LogLevel)
	}
	if cfg.MaxThreads != defaultConfig.MaxThreads {
		t.Errorf("expected default MaxThreads %d, got %d", defaultConfig.MaxThreads, cfg.MaxThreads)
	}
	if cfg.Apps == nil {
		t.Error("expected Apps to be non-nil slice")
	}
	if len(cfg.RefreshCommands) != len(defaultConfig.RefreshCommands) {
		t.Errorf("expected default RefreshCommands, got %v", cfg.RefreshCommands)
	}
}

func TestPostProcessConfig_MaxThreadsCapped(t *testing.T) {
	cfg := &Config{MaxThreads: 200, LogLevel: "info", RefreshCommands: [][]string{{}}}
	postProcessConfig(cfg)
	if cfg.MaxThreads != 100 {
		t.Errorf("expected MaxThreads capped at 100, got %d", cfg.MaxThreads)
	}
}

func TestPostProcessConfig_MaxThreadsZeroUsesDefault(t *testing.T) {
	cfg := &Config{MaxThreads: 0, LogLevel: "info", RefreshCommands: [][]string{{}}}
	postProcessConfig(cfg)
	if cfg.MaxThreads != defaultConfig.MaxThreads {
		t.Errorf("expected default MaxThreads %d, got %d", defaultConfig.MaxThreads, cfg.MaxThreads)
	}
}

func TestPostProcessConfig_NegativeMaxThreadsUsesDefault(t *testing.T) {
	cfg := &Config{MaxThreads: -5, LogLevel: "info", RefreshCommands: [][]string{{}}}
	postProcessConfig(cfg)
	if cfg.MaxThreads != defaultConfig.MaxThreads {
		t.Errorf("expected default MaxThreads %d, got %d", defaultConfig.MaxThreads, cfg.MaxThreads)
	}
}

func TestPostProcessConfig_EmptyLogLevelUsesDefault(t *testing.T) {
	cfg := &Config{LogLevel: "", MaxThreads: 3, RefreshCommands: [][]string{{}}}
	postProcessConfig(cfg)
	if cfg.LogLevel != defaultConfig.LogLevel {
		t.Errorf("expected default LogLevel %q, got %q", defaultConfig.LogLevel, cfg.LogLevel)
	}
}

func TestPostProcessConfig_NilAppsBecomesEmpty(t *testing.T) {
	cfg := &Config{LogLevel: "info", MaxThreads: 3, RefreshCommands: [][]string{{}}}
	postProcessConfig(cfg)
	if cfg.Apps == nil {
		t.Error("expected Apps to be non-nil after postProcessConfig")
	}
}

func TestPostProcessConfig_NilRefreshCommandsCopiesDefault(t *testing.T) {
	cfg := &Config{LogLevel: "info", MaxThreads: 3}
	postProcessConfig(cfg)

	if len(cfg.RefreshCommands) != len(defaultConfig.RefreshCommands) {
		t.Fatalf("expected %d refresh commands, got %d", len(defaultConfig.RefreshCommands), len(cfg.RefreshCommands))
	}

	// verify it's a copy, not a shared backing array
	cfg.RefreshCommands[0] = []string{"mutated"}
	if defaultConfig.RefreshCommands[0][0] == "mutated" {
		t.Error("postProcessConfig shared RefreshCommands backing array with defaultConfig")
	}
}

func TestPostProcessConfig_CommandTimeoutEmptyUsesDefault(t *testing.T) {
	cfg := &Config{LogLevel: "info", MaxThreads: 3, RefreshCommands: [][]string{{}}}
	postProcessConfig(cfg)
	if cfg.CommandTimeout != defaultCommandTimeout {
		t.Errorf("expected default CommandTimeout %q, got %q", defaultCommandTimeout, cfg.CommandTimeout)
	}
	if cfg.CommandTimeoutDuration != defaultCommandTimeoutDuration {
		t.Errorf("expected default CommandTimeoutDuration %s, got %s", defaultCommandTimeoutDuration, cfg.CommandTimeoutDuration)
	}
}

func TestPostProcessConfig_CommandTimeoutInvalidUsesDefault(t *testing.T) {
	cfg := &Config{LogLevel: "info", MaxThreads: 3, RefreshCommands: [][]string{{}}, CommandTimeout: "not-a-duration"}
	postProcessConfig(cfg)
	if cfg.CommandTimeoutDuration != defaultCommandTimeoutDuration {
		t.Errorf("expected default CommandTimeoutDuration %s, got %s", defaultCommandTimeoutDuration, cfg.CommandTimeoutDuration)
	}
}

func TestPostProcessConfig_CommandTimeoutNonPositiveUsesDefault(t *testing.T) {
	cfg := &Config{LogLevel: "info", MaxThreads: 3, RefreshCommands: [][]string{{}}, CommandTimeout: "0s"}
	postProcessConfig(cfg)
	if cfg.CommandTimeoutDuration != defaultCommandTimeoutDuration {
		t.Errorf("expected default CommandTimeoutDuration %s, got %s", defaultCommandTimeoutDuration, cfg.CommandTimeoutDuration)
	}
}

func TestPostProcessConfig_CommandTimeoutValid(t *testing.T) {
	cfg := &Config{LogLevel: "info", MaxThreads: 3, RefreshCommands: [][]string{{}}, CommandTimeout: "90s"}
	postProcessConfig(cfg)
	if cfg.CommandTimeoutDuration != 90*time.Second {
		t.Errorf("expected CommandTimeoutDuration 90s, got %s", cfg.CommandTimeoutDuration)
	}
}
