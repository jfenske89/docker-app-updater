package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/jfenske89/docker-app-updater/internal/discovery"
	"github.com/jfenske89/docker-app-updater/internal/update"
)

func result(name string, status update.Status, err error) update.Result {
	return update.Result{App: discovery.App{Name: name}, Status: status, Err: err}
}

func TestBuild_EmptyWhenNothingChanged(t *testing.T) {
	results := []update.Result{
		result("jellyfin", update.StatusUnchanged, nil),
		result("komga", update.StatusUnchanged, nil),
	}

	if got := Build("", results); got != "" {
		t.Errorf("Build() = %q, want empty string", got)
	}
}

func TestBuild_EmptyWhenNothingChanged_EvenWithLabel(t *testing.T) {
	results := []update.Result{
		result("jellyfin", update.StatusUnchanged, nil),
	}

	if got := Build("homelab", results); got != "" {
		t.Errorf("Build() = %q, want empty string", got)
	}
}

func TestBuild_GroupsByStatus(t *testing.T) {
	results := []update.Result{
		result("immich", update.StatusUpdated, nil),
		result("forgejo", update.StatusUpdated, nil),
		result("angie", update.StatusRecreated, nil),
		result("pihole", update.StatusRestarted, nil),
		result("backrest", update.StatusError, errors.New("pull: context deadline exceeded")),
		result("komga", update.StatusUnchanged, nil),
	}

	got := Build("", results)

	for _, want := range []string{
		"Docker apps updated (2)",
		"- forgejo",
		"- immich",
		"Recreated, image unchanged (1)",
		"- angie",
		"Restarted (1)",
		"- pihole",
		"⚠ Errors (1)",
		"- backrest",
		"context deadline exceeded",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Build() missing %q in:\n%s", want, got)
		}
	}

	if strings.Contains(got, "komga") {
		t.Errorf("Build() should not mention unchanged apps, got:\n%s", got)
	}
}

func TestBuild_PrependsLabel(t *testing.T) {
	results := []update.Result{
		result("audiobookshelf", update.StatusRecreated, nil),
	}

	got := Build("homelab", results)

	if !strings.HasPrefix(got, "**homelab**\n\n") {
		t.Errorf("Build() = %q, want to start with label header", got)
	}
}
