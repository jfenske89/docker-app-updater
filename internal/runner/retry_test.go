package runner

import (
	"errors"
	"testing"
	"time"
)

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		name   string
		output string
		err    error
		want   bool
	}{
		{
			name:   "toomanyrequests",
			output: "Error toomanyrequests: retry-after: 609.219µs, allowed: 44000/minute",
			err:    errors.New("exit status 1"),
			want:   true,
		},
		{
			name: "production ghcr example",
			output: "Image ghcr.io/getarcaneapp/manager:latest Pulling \n Image ghcr.io/getarcaneapp/manager:latest " +
				"Error toomanyrequests: retry-after: 609.219µs, allowed: 44000/minute\n" +
				"Error response from daemon: toomanyrequests: retry-after: 609.219µs, allowed: 44000/minute\n",
			err:  errors.New("exit status 1"),
			want: true,
		},
		{
			name:   "429 too many requests",
			output: "server responded 429 Too Many Requests",
			err:    errors.New("exit status 1"),
			want:   true,
		},
		{
			name:   "rate limit phrase",
			output: "you have hit a rate limit, please wait",
			err:    errors.New("exit status 1"),
			want:   true,
		},
		{
			name:   "case different",
			output: "Error TOOMANYREQUESTS: slow down",
			err:    errors.New("exit status 1"),
			want:   true,
		},
		{
			name:   "non-matching error",
			output: "no such file or directory",
			err:    errors.New("exit status 127"),
			want:   false,
		},
		{
			name:   "nil error",
			output: "toomanyrequests",
			err:    nil,
			want:   false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isRetryable(c.output, c.err); got != c.want {
				t.Errorf("isRetryable(%q, %v) = %v, want %v", c.output, c.err, got, c.want)
			}
		})
	}
}

func TestBackoffDelay(t *testing.T) {
	cases := []struct {
		base    time.Duration
		attempt int
	}{
		{time.Second, 1},
		{time.Second, 2},
		{time.Second, 5},
		{5 * time.Second, 1},
		{5 * time.Second, 10},
		{time.Hour, 3},
	}

	for _, c := range cases {
		delay := backoffDelay(c.base, c.attempt)
		if delay <= 0 {
			t.Errorf("backoffDelay(%s, %d) = %s, want > 0", c.base, c.attempt, delay)
		}
		if delay > maxRetryDelay {
			t.Errorf("backoffDelay(%s, %d) = %s, want <= %s", c.base, c.attempt, delay, maxRetryDelay)
		}
	}
}
