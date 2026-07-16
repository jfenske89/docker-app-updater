package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfenske89/docker-app-updater/internal/config"
)

func mkComposeProject(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDiscover_FindsComposeProjectsAndAppliesExcludesAndOverrides(t *testing.T) {
	root := t.TempDir()
	mkComposeProject(t, root, "jellyfin")
	mkComposeProject(t, root, "pihole")

	// Not a compose project - no compose file - should be ignored.
	if err := os.MkdirAll(filepath.Join(root, "not-an-app"), 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Discovery: config.Discovery{
			Roots:            []string{root},
			ComposeFilenames: []string{"docker-compose.yml"},
		},
		RefreshCommands: [][]string{{"docker", "compose", "pull"}},
		Excludes:        []string{"pihole"},
		Overrides: map[string]config.Override{
			"jellyfin": {Profiles: []string{"heavy"}, SkipIfNoContainers: new(false)},
		},
	}

	apps, err := Discover(cfg)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(apps) != 1 {
		t.Fatalf("expected 1 app (pihole excluded, not-an-app has no compose file), got %d: %+v", len(apps), apps)
	}

	app := apps[0]
	if app.Name != "jellyfin" {
		t.Errorf("Name = %q, want jellyfin", app.Name)
	}
	if len(app.Profiles) != 1 || app.Profiles[0] != "heavy" {
		t.Errorf("Profiles = %v, want [heavy] (override should have applied)", app.Profiles)
	}
	if len(app.RefreshCommands) == 0 {
		t.Error("expected app to inherit global refresh_commands")
	}
	if app.SkipIfNoContainers == nil || *app.SkipIfNoContainers {
		t.Errorf("SkipIfNoContainers = %v, want a pointer to false (override should have applied)", app.SkipIfNoContainers)
	}
}

func TestDiscover_ExplicitAppOutsideRoots(t *testing.T) {
	cfg := config.Config{
		RefreshCommands: [][]string{{"docker", "compose", "pull"}},
		Apps: []config.App{
			{Name: "manual-app", Path: "/opt/manual-app", RefreshCommands: [][]string{{"./refresh.sh"}}},
		},
	}

	apps, err := Discover(cfg)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Path != "/opt/manual-app" {
		t.Errorf("Path = %q, want /opt/manual-app", apps[0].Path)
	}
	if len(apps[0].RefreshCommands) != 1 || apps[0].RefreshCommands[0][0] != "./refresh.sh" {
		t.Errorf("expected explicit app's own refresh_commands to be kept, got %v", apps[0].RefreshCommands)
	}
}

func TestFilterByProfile(t *testing.T) {
	apps := []App{
		{Name: "untagged"},
		{Name: "heavy-only", Profiles: []string{"heavy"}},
		{Name: "gpu-only", Profiles: []string{"gpu"}},
	}

	t.Run("no profile flag runs only untagged apps", func(t *testing.T) {
		got := FilterByProfile(apps, "")
		if len(got) != 1 || got[0].Name != "untagged" {
			t.Errorf("FilterByProfile(apps, \"\") = %+v, want just [untagged]", got)
		}
	})

	t.Run("matching profile includes untagged plus that tag", func(t *testing.T) {
		got := FilterByProfile(apps, "heavy")
		names := map[string]bool{}
		for _, a := range got {
			names[a.Name] = true
		}
		if !names["untagged"] || !names["heavy-only"] || names["gpu-only"] {
			t.Errorf("FilterByProfile(apps, \"heavy\") = %+v, want [untagged heavy-only]", got)
		}
	})
}
