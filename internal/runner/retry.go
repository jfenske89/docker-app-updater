package runner

import (
	"math/rand/v2"
	"strings"
	"time"
)

// RetryOptions controls how Run retries a command whose output looks like a
// transient registry rate-limit response. MaxAttempts <1 is treated as 1
// (no retry), so a zero-value RetryOptions{} behaves like no retry support
// existed.
type RetryOptions struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

const maxRetryDelay = 60 * time.Second

// retryablePatterns are matched case-insensitively against combined command
// output+error. Deliberately excludes a bare "429" (false-positive risk on
// unrelated numbers) and deliberately does not parse any registry-provided
// retry-after value (observed to be unreliable, e.g.
// "retry-after: 609.219µs").
var retryablePatterns = []string{
	"toomanyrequests",
	"429 too many requests",
	"rate limit",
}

func isRetryable(output string, err error) bool {
	if err == nil {
		return false
	}
	haystack := strings.ToLower(output + " " + err.Error())
	for _, p := range retryablePatterns {
		if strings.Contains(haystack, p) {
			return true
		}
	}
	return false
}

// backoffDelay returns the exponential-backoff-with-jitter wait before the
// next attempt, capped at maxRetryDelay both before and after jitter.
func backoffDelay(base time.Duration, attempt int) time.Duration {
	delay := base * (1 << (attempt - 1))
	if delay > maxRetryDelay || delay <= 0 {
		delay = maxRetryDelay
	}
	jittered := time.Duration(float64(delay) * (0.5 + rand.Float64())) //nolint:gosec // G404: jitter timing, not security-sensitive
	return min(jittered, maxRetryDelay)
}
