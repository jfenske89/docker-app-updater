// Package dockercli wraps the "docker compose" subcommands used to
// fingerprint a project's containers and images, so update detection can
// compare real Docker state instead of inferring it from container uptime
// text.
package dockercli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jfenske89/docker-app-updater/internal/exec"
)

// container is one line of "docker compose ps -a --format json" output.
type container struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"` // "running", "exited", "created", ...
}

// image is one line of "docker compose images --format json" output.
type image struct {
	ID            string `json:"ID"` // image ID
	ContainerName string `json:"ContainerName"`
	Repository    string `json:"Repository"`
	Tag           string `json:"Tag"`
}

// ServiceState is a compose service's identity at a point in time: which
// container is running it, on which image, and whether it's up.
type ServiceState struct {
	Service     string
	ContainerID string
	ImageID     string
	Running     bool
}

// Snapshot captures the current container/image identity of every service
// in the compose project at dir, keyed by service name.
func Snapshot(ctx context.Context, executor exec.Executor, dir string) (map[string]ServiceState, error) {
	containers, err := listContainers(ctx, executor, dir)
	if err != nil {
		return nil, err
	}

	images, err := listImages(ctx, executor, dir)
	if err != nil {
		return nil, err
	}

	imageByContainerName := make(map[string]image, len(images))
	for _, img := range images {
		imageByContainerName[img.ContainerName] = img
	}

	states := make(map[string]ServiceState, len(containers))
	for _, c := range containers {
		img := imageByContainerName[c.Name]
		states[c.Service] = ServiceState{
			Service:     c.Service,
			ContainerID: c.ID,
			ImageID:     img.ID,
			Running:     c.State == "running",
		}
	}

	return states, nil
}

// Pull runs "docker compose pull" in dir.
func Pull(ctx context.Context, executor exec.Executor, dir string) (string, error) {
	output, err := executor(ctx, "docker", []string{"compose", "pull"}, dir)
	if err != nil {
		return output, fmt.Errorf("docker compose pull: %w: %s", err, strings.TrimSpace(output))
	}
	return output, nil
}

func listContainers(ctx context.Context, executor exec.Executor, dir string) ([]container, error) {
	output, err := executor(ctx, "docker", []string{"compose", "ps", "-a", "--format", "json"}, dir)
	if err != nil {
		return nil, fmt.Errorf("docker compose ps: %w: %s", err, strings.TrimSpace(output))
	}

	var containers []container
	if err := unmarshalJSONLines(output, &containers); err != nil {
		return nil, fmt.Errorf("failed to parse docker compose ps output: %w", err)
	}
	return containers, nil
}

func listImages(ctx context.Context, executor exec.Executor, dir string) ([]image, error) {
	output, err := executor(ctx, "docker", []string{"compose", "images", "--format", "json"}, dir)
	if err != nil {
		return nil, fmt.Errorf("docker compose images: %w: %s", err, strings.TrimSpace(output))
	}

	var images []image
	if err := unmarshalJSONLines(output, &images); err != nil {
		return nil, fmt.Errorf("failed to parse docker compose images output: %w", err)
	}
	return images, nil
}

// unmarshalJSONLines decodes docker compose's "--format json" output into
// dst (a pointer to a slice), accepting either a single JSON array (older
// compose) or newline-delimited JSON objects (newer compose) since the shape
// has changed across compose versions.
func unmarshalJSONLines[T any](output string, dst *[]T) error {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}

	if trimmed[0] == '[' {
		return json.Unmarshal([]byte(trimmed), dst)
	}

	var items []T
	for line := range strings.SplitSeq(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return fmt.Errorf("line %q: %w", line, err)
		}
		items = append(items, item)
	}
	*dst = items
	return nil
}
