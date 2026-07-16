package update

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jfenske89/docker-app-updater/internal/config"
	"github.com/jfenske89/docker-app-updater/internal/discovery"
)

// fakeExecutor simulates "docker compose ps"/"images" returning a "before"
// snapshot on the first call and an "after" snapshot on the second, and
// otherwise succeeds (for refresh/after commands) unless failOn matches.
func fakeExecutor(t *testing.T, beforePS, afterPS, beforeImages, afterImages string, failOn string) func(ctx context.Context, name string, args []string, dir string) (string, error) {
	t.Helper()
	calls := map[string]int{}

	return func(ctx context.Context, name string, args []string, dir string) (string, error) {
		joined := name + " " + strings.Join(args, " ")

		if failOn != "" && strings.Contains(joined, failOn) {
			return "boom", errors.New("simulated failure")
		}

		switch {
		case strings.Contains(joined, "ps -a"):
			calls["ps"]++
			if calls["ps"] == 1 {
				return beforePS, nil
			}
			return afterPS, nil
		case strings.Contains(joined, "compose images"):
			calls["images"]++
			if calls["images"] == 1 {
				return beforeImages, nil
			}
			return afterImages, nil
		default:
			return "", nil
		}
	}
}

func testApp() discovery.App {
	return discovery.App{
		Name: "web",
		Path: "/apps/web",
		RefreshCommands: [][]string{
			{"docker", "compose", "pull"},
			{"docker", "compose", "up", "-d", "--remove-orphans"},
		},
	}
}

func testConfig() config.Config {
	return config.Config{CommandTimeoutDuration: 5 * time.Second}
}

func TestRun_DetectsUpdate(t *testing.T) {
	executor := fakeExecutor(
		t,
		`{"ID":"c1","Name":"web-1","Service":"web","State":"running"}`,
		`{"ID":"c2","Name":"web-1","Service":"web","State":"running"}`,
		`{"ID":"i1","ContainerName":"web-1","Repository":"nginx","Tag":"latest"}`,
		`{"ID":"i2","ContainerName":"web-1","Repository":"nginx","Tag":"latest"}`,
		"",
	)

	result := Run(context.Background(), testApp(), testConfig(), executor)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Status != StatusUpdated {
		t.Errorf("Status = %s, want %s", result.Status, StatusUpdated)
	}
}

func TestRun_NoChange(t *testing.T) {
	ps := `{"ID":"c1","Name":"web-1","Service":"web","State":"running"}`
	images := `{"ID":"i1","ContainerName":"web-1","Repository":"nginx","Tag":"latest"}`
	executor := fakeExecutor(t, ps, ps, images, images, "")

	result := Run(context.Background(), testApp(), testConfig(), executor)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Status != StatusUnchanged {
		t.Errorf("Status = %s, want %s", result.Status, StatusUnchanged)
	}
}

func TestRun_RefreshCommandFails(t *testing.T) {
	ps := `{"ID":"c1","Name":"web-1","Service":"web","State":"running"}`
	images := `{"ID":"i1","ContainerName":"web-1","Repository":"nginx","Tag":"latest"}`
	executor := fakeExecutor(t, ps, ps, images, images, "compose pull")

	result := Run(context.Background(), testApp(), testConfig(), executor)

	if result.Err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Status != StatusError {
		t.Errorf("Status = %s, want %s", result.Status, StatusError)
	}
}

func TestRun_DryRunSkipsCommandsAndReportsUnchanged(t *testing.T) {
	calls := 0
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		joined := name + " " + strings.Join(args, " ")
		if strings.Contains(joined, "compose pull") || strings.Contains(joined, "compose up") {
			calls++
		}
		return `{"ID":"c1","Name":"web-1","Service":"web","State":"running"}`, nil
	}

	cfg := testConfig()
	cfg.DryRun = true

	result := Run(context.Background(), testApp(), cfg, executor)

	if calls != 0 {
		t.Errorf("expected refresh commands not to execute in dry run, got %d calls", calls)
	}
	if result.Status != StatusUnchanged {
		t.Errorf("Status = %s, want %s", result.Status, StatusUnchanged)
	}
}

// noContainersExecutor simulates an app with an empty compose project: "ps"
// and "images" both report nothing, and refreshCalled records whether the
// refresh commands were reached.
func noContainersExecutor(refreshCalled *bool) func(ctx context.Context, name string, args []string, dir string) (string, error) {
	return func(ctx context.Context, name string, args []string, dir string) (string, error) {
		joined := name + " " + strings.Join(args, " ")
		if strings.Contains(joined, "compose pull") || strings.Contains(joined, "compose up") {
			*refreshCalled = true
		}
		return "", nil
	}
}

func TestRun_SkipsAppsWithoutContainersByDefault(t *testing.T) {
	var refreshCalled bool
	result := Run(context.Background(), testApp(), testConfig(), noContainersExecutor(&refreshCalled))

	if result.Status != StatusSkipped {
		t.Errorf("Status = %s, want %s", result.Status, StatusSkipped)
	}
	if refreshCalled {
		t.Error("expected refresh commands not to run for an app with no containers")
	}
}

func TestRun_GlobalSkipIfNoContainersFalseStillRuns(t *testing.T) {
	var refreshCalled bool
	skip := false
	cfg := testConfig()
	cfg.SkipIfNoContainers = &skip

	result := Run(context.Background(), testApp(), cfg, noContainersExecutor(&refreshCalled))

	if result.Status == StatusSkipped {
		t.Error("expected the app to run despite having no containers, since skip_if_no_containers is false")
	}
	if !refreshCalled {
		t.Error("expected refresh commands to run")
	}
}

func TestRun_AppOverrideForcesRunEvenWithoutContainers(t *testing.T) {
	var refreshCalled bool
	skip := false
	app := testApp()
	app.SkipIfNoContainers = &skip // per-app override wins over the global default

	result := Run(context.Background(), app, testConfig(), noContainersExecutor(&refreshCalled))

	if result.Status == StatusSkipped {
		t.Error("expected the per-app override to force a run despite having no containers")
	}
	if !refreshCalled {
		t.Error("expected refresh commands to run")
	}
}
