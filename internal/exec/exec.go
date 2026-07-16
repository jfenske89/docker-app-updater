// Package exec defines the command-execution seam used throughout the
// codebase, so tests can substitute a fake in place of the real
// os/exec-backed implementation.
package exec

import (
	"context"
	"os/exec"
)

// Executor runs a single command in dir and returns its combined output.
type Executor func(ctx context.Context, name string, args []string, dir string) (string, error)

// OS is the real Executor, backed by os/exec.
func OS(ctx context.Context, name string, args []string, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: name/args are the configured refresh/after commands, not untrusted input
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}
