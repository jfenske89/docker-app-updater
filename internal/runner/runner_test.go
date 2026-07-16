package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRun_SubstitutesVariables(t *testing.T) {
	t.Setenv("HOME", "/home/tester")

	var gotName string
	var gotArgs []string
	var gotDir string
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		gotName = name
		gotArgs = args
		gotDir = dir
		return "ok", nil
	}

	_, err := Run(
		context.Background(),
		executor,
		[]string{"{{app.path}}/refresh.sh", "--name={{app.name}}", "--home={{HOME}}"},
		"/apps/jellyfin",
		"jellyfin",
		time.Second,
		false,
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if gotName != "/apps/jellyfin/refresh.sh" {
		t.Errorf("name = %q", gotName)
	}
	if gotDir != "/apps/jellyfin" {
		t.Errorf("dir = %q", gotDir)
	}
	wantArgs := []string{"--name=jellyfin", "--home=/home/tester"}
	if strings.Join(gotArgs, ",") != strings.Join(wantArgs, ",") {
		t.Errorf("args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestRun_DryRunDoesNotExecute(t *testing.T) {
	called := false
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		called = true
		return "", nil
	}

	_, err := Run(context.Background(), executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, true)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if called {
		t.Error("expected executor not to be called in dry run mode")
	}
}

func TestRun_WrapsExecutorError(t *testing.T) {
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		return "some output", errors.New("exit status 1")
	}

	_, err := Run(context.Background(), executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "docker compose pull") || !strings.Contains(err.Error(), "some output") {
		t.Errorf("error = %v, want it to mention the command and output", err)
	}
}

func TestRunAll_StopsAtFirstError(t *testing.T) {
	var ran []string
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		ran = append(ran, name)
		if name == "fail" {
			return "", errors.New("boom")
		}
		return "", nil
	}

	cmds := [][]string{{"ok-1"}, {"fail"}, {"ok-2"}}
	err := RunAll(context.Background(), executor, cmds, "/apps/x", "x", time.Second, false)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Join(ran, ",") != "ok-1,fail" {
		t.Errorf("ran = %v, want RunAll to stop after the failing command", ran)
	}
}
