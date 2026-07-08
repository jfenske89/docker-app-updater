package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReplaceVariables(t *testing.T) {
	tests := []struct {
		input    string
		path     string
		appName  string
		expected string
	}{
		{"{{app.name}}", "/some/path", "myapp", "myapp"},
		{"{{app.path}}", "/some/path", "myapp", "/some/path"},
		{"{{app.name}}-{{app.path}}", "/some/path", "myapp", "myapp-/some/path"},
		{"no-variables", "/some/path", "myapp", "no-variables"},
		{"", "", "", ""},
	}

	for _, tt := range tests {
		got := replaceVariables(tt.input, tt.path, tt.appName)
		if got != tt.expected {
			t.Errorf("replaceVariables(%q, %q, %q) = %q, want %q",
				tt.input, tt.path, tt.appName, got, tt.expected)
		}
	}
}

func TestExecuteCommand_DryRun(t *testing.T) {
	called := false
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		called = true
		return "", nil
	}

	cfg := Config{DryRun: true}
	output, err := executeCommand([]string{"echo", "hello"}, "/tmp", "testapp", cfg, fakeExecutor)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output != "***DRY RUN***" {
		t.Errorf("expected dry run sentinel, got %q", output)
	}
	if called {
		t.Error("executor must not be called in dry run mode")
	}
}

func TestExecuteCommand_Success(t *testing.T) {
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		return "hello\n", nil
	}

	cfg := Config{DryRun: false}
	output, err := executeCommand([]string{"echo", "hello"}, "/tmp", "testapp", cfg, fakeExecutor)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output != "hello\n" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestExecuteCommand_Failure(t *testing.T) {
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		return "some output\n", errors.New("exit status 1")
	}

	cfg := Config{DryRun: false}
	output, err := executeCommand([]string{"false"}, "/tmp", "testapp", cfg, fakeExecutor)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if output != "some output\n" {
		t.Errorf("unexpected output: %q", output)
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("error should contain original message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "false") {
		t.Errorf("error should contain command name, got: %v", err)
	}
}

func TestExecuteCommand_VariableSubstitution(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	var capturedDir string

	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		capturedName = name
		capturedArgs = args
		capturedDir = dir
		return "", nil
	}

	cfg := Config{DryRun: false}
	_, err := executeCommand(
		[]string{"docker", "compose", "-f", "{{app.path}}/docker-compose.yml", "--project-name", "{{app.name}}"},
		"/my/app",
		"myapp",
		cfg,
		fakeExecutor,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedName != "docker" {
		t.Errorf("expected command=docker, got %q", capturedName)
	}
	if capturedDir != "/my/app" {
		t.Errorf("expected dir=/my/app, got %q", capturedDir)
	}

	wantArgs := []string{"compose", "-f", "/my/app/docker-compose.yml", "--project-name", "myapp"}
	if len(capturedArgs) != len(wantArgs) {
		t.Fatalf("expected %d args %v, got %d %v", len(wantArgs), wantArgs, len(capturedArgs), capturedArgs)
	}
	for i, want := range wantArgs {
		if capturedArgs[i] != want {
			t.Errorf("arg[%d]: expected %q, got %q", i, want, capturedArgs[i])
		}
	}
}

func TestExecuteCommand_WrapsUnderlyingError(t *testing.T) {
	sentinel := errors.New("boom")
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		return "", sentinel
	}

	cfg := Config{DryRun: false, CommandTimeoutDuration: time.Second}
	_, err := executeCommand([]string{"echo", "hello"}, "/tmp", "testapp", cfg, fakeExecutor)

	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got: %v", err)
	}
}

func TestExecuteCommand_TimesOutLongRunningCommand(t *testing.T) {
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}

	cfg := Config{DryRun: false, CommandTimeoutDuration: 20 * time.Millisecond}
	_, err := executeCommand([]string{"sleep", "60"}, "/tmp", "testapp", cfg, fakeExecutor)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected wrapped context.DeadlineExceeded, got: %v", err)
	}
}

func TestUpdateApp_RunsRefreshThenAfterCommands(t *testing.T) {
	var executed []string
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		executed = append(executed, name)
		return "", nil
	}

	app := App{Name: "myapp", Path: "/app", AfterCommands: [][]string{{"notify"}}}
	cfg := Config{RefreshCommands: [][]string{{"pull"}, {"up"}}}

	if err := updateApp(app, cfg, fakeExecutor); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(executed) != 3 {
		t.Fatalf("expected 3 commands, got %d: %v", len(executed), executed)
	}
	want := []string{"pull", "up", "notify"}
	for i, w := range want {
		if executed[i] != w {
			t.Errorf("command[%d]: expected %q, got %q", i, w, executed[i])
		}
	}
}

func TestUpdateApp_StopsOnFirstError(t *testing.T) {
	callCount := 0
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		callCount++
		if name == "up" {
			return "", errors.New("container failed to start")
		}
		return "", nil
	}

	app := App{Name: "myapp", Path: "/app", AfterCommands: [][]string{{"notify"}}}
	cfg := Config{RefreshCommands: [][]string{{"pull"}, {"up"}}}

	err := updateApp(app, cfg, fakeExecutor)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount != 2 {
		t.Errorf("expected 2 executor calls (pull + up), got %d", callCount)
	}
	if !strings.Contains(err.Error(), "container failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUpdateApp_DoesNotMutateRefreshCommands(t *testing.T) {
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		return "", nil
	}

	app := App{Name: "myapp", Path: "/app", AfterCommands: [][]string{{"notify"}}}
	cfg := Config{RefreshCommands: [][]string{{"pull"}, {"up"}}}
	originalLen := len(cfg.RefreshCommands)

	_ = updateApp(app, cfg, fakeExecutor)

	if len(cfg.RefreshCommands) != originalLen {
		t.Errorf("updateApp mutated RefreshCommands: got len %d, want %d",
			len(cfg.RefreshCommands), originalLen)
	}
}

func TestUpdateApp_NoAfterCommands(t *testing.T) {
	var executed []string
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		executed = append(executed, name)
		return "", nil
	}

	app := App{Name: "myapp", Path: "/app"}
	cfg := Config{RefreshCommands: [][]string{{"pull"}, {"up"}}}

	if err := updateApp(app, cfg, fakeExecutor); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(executed) != 2 {
		t.Errorf("expected 2 commands, got %d: %v", len(executed), executed)
	}
}

func TestFilterApps_ExcludesSkippedAppsPreservingOrder(t *testing.T) {
	apps := []App{
		{Name: "a"},
		{Name: "b", Skip: true},
		{Name: "c"},
		{Name: "d", Skip: true},
	}

	got := filterApps(apps)

	want := []string{"a", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d apps, got %d: %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("app[%d]: expected %q, got %q", i, w, got[i].Name)
		}
	}
}

func TestFilterApps_NoneSkipped(t *testing.T) {
	apps := []App{{Name: "a"}, {Name: "b"}}

	got := filterApps(apps)

	if len(got) != 2 {
		t.Fatalf("expected 2 apps, got %d: %v", len(got), got)
	}
}

func TestFilterApps_AllSkipped(t *testing.T) {
	apps := []App{{Name: "a", Skip: true}, {Name: "b", Skip: true}}

	got := filterApps(apps)

	if len(got) != 0 {
		t.Errorf("expected no apps, got %d: %v", len(got), got)
	}
}

func TestUpdateApp_AppRefreshCommandsOverrideGlobal(t *testing.T) {
	var executed []string
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		executed = append(executed, name)
		return "", nil
	}

	app := App{
		Name:            "myapp",
		Path:            "/app",
		RefreshCommands: [][]string{{"custom-pull"}, {"custom-up"}},
		AfterCommands:   [][]string{{"notify"}},
	}
	cfg := Config{RefreshCommands: [][]string{{"pull"}, {"up"}}}

	if err := updateApp(app, cfg, fakeExecutor); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	want := []string{"custom-pull", "custom-up", "notify"}
	if len(executed) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(executed), executed)
	}
	for i, w := range want {
		if executed[i] != w {
			t.Errorf("command[%d]: expected %q, got %q", i, w, executed[i])
		}
	}
}

func TestUpdateApp_EmptyAppRefreshCommandsInheritsGlobal(t *testing.T) {
	var executed []string
	fakeExecutor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		executed = append(executed, name)
		return "", nil
	}

	// An explicit but empty list inherits the global refresh_commands, same as omitting it.
	app := App{Name: "myapp", Path: "/app", RefreshCommands: [][]string{}}
	cfg := Config{RefreshCommands: [][]string{{"pull"}, {"up"}}}

	if err := updateApp(app, cfg, fakeExecutor); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	want := []string{"pull", "up"}
	if len(executed) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(executed), executed)
	}
	for i, w := range want {
		if executed[i] != w {
			t.Errorf("command[%d]: expected %q, got %q", i, w, executed[i])
		}
	}
}
