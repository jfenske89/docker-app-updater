// Package discovery finds docker compose projects on disk and merges them
// with explicit config entries, excludes, and overrides into a final app
// list ready to update.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/sirupsen/logrus"

	"github.com/jfenske89/docker-app-updater/internal/config"
)

// App is a fully resolved app: a name, a working directory, and the
// commands to run there.
type App struct {
	Name            string
	Path            string
	RefreshCommands [][]string
	AfterCommands   [][]string

	// Profiles is empty for an app that only runs when no --profile is
	// given. A non-empty list restricts the app to invocations whose
	// --profile matches one of these freeform tags.
	Profiles []string

	// SkipIfNoContainers overrides Config.SkipIfNoContainers for this app.
	// Nil means "inherit the global default".
	SkipIfNoContainers *bool
}

// Discover walks cfg.Discovery.Roots one level deep for compose projects,
// merges in cfg.Apps, applies cfg.Excludes, and applies cfg.Overrides.
func Discover(cfg config.Config) ([]App, error) {
	byName := make(map[string]App)
	excluded := toSet(cfg.Excludes)

	for _, root := range cfg.Discovery.Roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("failed to read discovery root %s: %w", root, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			path := filepath.Join(root, name)

			if !hasComposeFile(path, cfg.Discovery.ComposeFilenames) {
				continue
			}
			if excluded[name] {
				logrus.Debugf("[%s] excluded", name)
				continue
			}
			if _, exists := byName[name]; exists {
				logrus.Warnf("[%s] discovered again at %s, keeping first occurrence", name, path)
				continue
			}

			byName[name] = App{
				Name:            name,
				Path:            path,
				RefreshCommands: cfg.RefreshCommands,
			}
		}
	}

	for _, explicit := range cfg.Apps {
		if excluded[explicit.Name] {
			logrus.Debugf("[%s] excluded", explicit.Name)
			continue
		}

		refreshCommands := cfg.RefreshCommands
		if len(explicit.RefreshCommands) > 0 {
			refreshCommands = explicit.RefreshCommands
		}

		byName[explicit.Name] = App{
			Name:               explicit.Name,
			Path:               explicit.Path,
			RefreshCommands:    refreshCommands,
			AfterCommands:      explicit.AfterCommands,
			Profiles:           explicit.Profiles,
			SkipIfNoContainers: explicit.SkipIfNoContainers,
		}
	}

	for name, override := range cfg.Overrides {
		app, exists := byName[name]
		if !exists {
			logrus.Warnf("override for %q does not match any discovered or explicit app", name)
			continue
		}

		if len(override.RefreshCommands) > 0 {
			app.RefreshCommands = override.RefreshCommands
		}
		if len(override.AfterCommands) > 0 {
			app.AfterCommands = override.AfterCommands
		}
		if len(override.Profiles) > 0 {
			app.Profiles = override.Profiles
		}
		if override.SkipIfNoContainers != nil {
			app.SkipIfNoContainers = override.SkipIfNoContainers
		}

		byName[name] = app
	}

	apps := make([]App, 0, len(byName))
	for _, app := range byName {
		apps = append(apps, app)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })

	return apps, nil
}

// FilterByProfile returns the apps that should run for the given profile.
// Passing an empty profile runs only untagged apps. Passing a non-empty
// profile runs only apps whose Profiles list contains it — untagged apps
// are excluded in that case.
func FilterByProfile(apps []App, profile string) []App {
	result := make([]App, 0, len(apps))
	for _, app := range apps {
		if profile == "" {
			if len(app.Profiles) == 0 {
				result = append(result, app)
			}
			continue
		}
		if contains(app.Profiles, profile) {
			result = append(result, app)
		}
	}
	return result
}

func hasComposeFile(dir string, filenames []string) bool {
	for _, name := range filenames {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}
