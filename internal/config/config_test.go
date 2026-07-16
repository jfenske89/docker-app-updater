package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_AppliesDefaults(t *testing.T) {
	path := writeConfig(t, `
discovery:
  roots: [/opt/apps]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MaxThreads != defaultMaxThreads {
		t.Errorf("MaxThreads = %d, want default %d", cfg.MaxThreads, defaultMaxThreads)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.CommandTimeoutDuration != defaultCommandTimeoutDuration {
		t.Errorf("CommandTimeoutDuration = %s, want %s", cfg.CommandTimeoutDuration, defaultCommandTimeoutDuration)
	}
	if len(cfg.RefreshCommands) != 2 {
		t.Errorf("expected default refresh_commands, got %v", cfg.RefreshCommands)
	}
	if len(cfg.Discovery.ComposeFilenames) == 0 {
		t.Error("expected default compose_filenames to be populated")
	}
}

func TestLoad_ParsesOverridesAndGotify(t *testing.T) {
	path := writeConfig(t, `
max_threads: 10
command_timeout: 5m
discovery:
  roots: [/mnt/data/apps, /mnt/storage/apps]
excludes: [pihole]
overrides:
  audiobookshelf:
    refresh_commands: [["./refresh.sh"]]
  immich:
    profiles: [heavy]
gotify:
  url: "https://gotify.example.com"
  token: "abc123"
  priority: 5
  label: "homelab"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MaxThreads != 10 {
		t.Errorf("MaxThreads = %d, want 10", cfg.MaxThreads)
	}
	if cfg.CommandTimeoutDuration != 5*time.Minute {
		t.Errorf("CommandTimeoutDuration = %s, want 5m", cfg.CommandTimeoutDuration)
	}
	if len(cfg.Excludes) != 1 || cfg.Excludes[0] != "pihole" {
		t.Errorf("Excludes = %v", cfg.Excludes)
	}
	if override, ok := cfg.Overrides["immich"]; !ok || len(override.Profiles) != 1 || override.Profiles[0] != "heavy" {
		t.Errorf("Overrides[immich] = %+v", cfg.Overrides["immich"])
	}
	if cfg.Gotify.URL != "https://gotify.example.com" || cfg.Gotify.Token != "abc123" || cfg.Gotify.Priority != 5 || cfg.Gotify.Label != "homelab" {
		t.Errorf("Gotify = %+v", cfg.Gotify)
	}
}

func TestLoad_InvalidTimeoutFallsBackToDefault(t *testing.T) {
	path := writeConfig(t, `
command_timeout: "not-a-duration"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CommandTimeoutDuration != defaultCommandTimeoutDuration {
		t.Errorf("CommandTimeoutDuration = %s, want default %s", cfg.CommandTimeoutDuration, defaultCommandTimeoutDuration)
	}
}

func TestLoad_MissingFileReturnsError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml")); err == nil {
		t.Error("expected an error for a missing config file")
	}
}

func TestSkipsAppsWithoutContainers_DefaultsToTrue(t *testing.T) {
	cfg := Config{}
	if !cfg.SkipsAppsWithoutContainers() {
		t.Error("expected SkipsAppsWithoutContainers() to default to true when unset")
	}
}

func TestNormalizeMaxThreads(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, defaultMaxThreads},
		{-1, defaultMaxThreads},
		{10, 10},
		{200, 100},
	}
	for _, c := range cases {
		if got := NormalizeMaxThreads(c.in); got != c.want {
			t.Errorf("NormalizeMaxThreads(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNormalizeCommandTimeout(t *testing.T) {
	if s, d := NormalizeCommandTimeout(""); s != defaultCommandTimeout || d != defaultCommandTimeoutDuration {
		t.Errorf("NormalizeCommandTimeout(\"\") = (%q, %s), want default", s, d)
	}
	if s, d := NormalizeCommandTimeout("30s"); s != "30s" || d != 30*time.Second {
		t.Errorf("NormalizeCommandTimeout(\"30s\") = (%q, %s), want (30s, 30s)", s, d)
	}
	if s, d := NormalizeCommandTimeout("not-a-duration"); s != defaultCommandTimeout || d != defaultCommandTimeoutDuration {
		t.Errorf("NormalizeCommandTimeout(invalid) = (%q, %s), want default", s, d)
	}
}

func TestSkipsAppsWithoutContainers_HonorsExplicitFalse(t *testing.T) {
	path := writeConfig(t, `skip_if_no_containers: false`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SkipsAppsWithoutContainers() {
		t.Error("expected SkipsAppsWithoutContainers() to be false when explicitly set")
	}
}
