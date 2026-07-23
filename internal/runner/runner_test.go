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
		RetryOptions{},
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

	_, err := Run(context.Background(), executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, true, RetryOptions{})
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

	_, err := Run(context.Background(), executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, false, RetryOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "docker compose pull") || !strings.Contains(err.Error(), "some output") {
		t.Errorf("error = %v, want it to mention the command and output", err)
	}
}

func TestRun_RetriesOnRateLimitThenSucceeds(t *testing.T) {
	calls := 0
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		calls++
		if calls < 3 {
			return "Error toomanyrequests: retry-after: 609.219µs", errors.New("exit status 1")
		}
		return "ok", nil
	}

	retry := RetryOptions{MaxAttempts: 5, BaseDelay: time.Millisecond}
	output, err := Run(context.Background(), executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, false, retry)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output != "ok" {
		t.Errorf("output = %q, want %q", output, "ok")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRun_DoesNotRetryNonMatchingError(t *testing.T) {
	calls := 0
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		calls++
		return "no such file or directory", errors.New("exit status 127")
	}

	retry := RetryOptions{MaxAttempts: 5, BaseDelay: time.Millisecond}
	_, err := Run(context.Background(), executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, false, retry)
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (non-matching error should not be retried)", calls)
	}
}

func TestRun_ExhaustsRetriesReturnsFinalError(t *testing.T) {
	calls := 0
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		calls++
		return "Error toomanyrequests: slow down", errors.New("exit status 1")
	}

	retry := RetryOptions{MaxAttempts: 3, BaseDelay: time.Millisecond}
	_, err := Run(context.Background(), executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, false, retry)
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (MaxAttempts)", calls)
	}
	if !strings.Contains(err.Error(), "docker compose pull") {
		t.Errorf("error = %v, want it to mention the command", err)
	}
}

func TestRun_ContextCancelDuringBackoffAbortsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		calls++
		if calls == 1 {
			go cancel()
		}
		return "Error toomanyrequests: slow down", errors.New("exit status 1")
	}

	retry := RetryOptions{MaxAttempts: 5, BaseDelay: 5 * time.Second}
	start := time.Now()
	_, err := Run(ctx, executor, []string{"docker", "compose", "pull"}, "/apps/x", "x", time.Second, false, retry)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want it to wrap context.Canceled", err)
	}
	if elapsed >= 5*time.Second {
		t.Errorf("Run() took %s, want it to return promptly after context cancellation", elapsed)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should abort during backoff before a second attempt)", calls)
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
	err := RunAll(context.Background(), executor, cmds, "/apps/x", "x", time.Second, false, RetryOptions{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Join(ran, ",") != "ok-1,fail" {
		t.Errorf("ran = %v, want RunAll to stop after the failing command", ran)
	}
}
