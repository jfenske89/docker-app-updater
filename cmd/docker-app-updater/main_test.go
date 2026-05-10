package main

import (
	"errors"
	"strings"
	"testing"
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
	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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
	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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
	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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

	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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

func TestUpdateApp_RunsRefreshThenAfterCommands(t *testing.T) {
	var executed []string
	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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
	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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
	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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
	fakeExecutor := func(name string, args []string, dir string) (string, error) {
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
