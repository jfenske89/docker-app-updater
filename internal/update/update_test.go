package update

import (
	"testing"

	"github.com/jfenske89/docker-app-updater/internal/dockercli"
)

func state(containerID, imageID string, running bool) dockercli.ServiceState {
	return dockercli.ServiceState{ContainerID: containerID, ImageID: imageID, Running: running}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name   string
		before map[string]dockercli.ServiceState
		after  map[string]dockercli.ServiceState
		want   Status
	}{
		{
			name:   "unchanged",
			before: map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			after:  map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			want:   StatusUnchanged,
		},
		{
			name:   "new container new image is an update",
			before: map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			after:  map[string]dockercli.ServiceState{"web": state("c2", "i2", true)},
			want:   StatusUpdated,
		},
		{
			name:   "new container same image is a recreate",
			before: map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			after:  map[string]dockercli.ServiceState{"web": state("c2", "i1", true)},
			want:   StatusRecreated,
		},
		{
			name:   "same container, was down and is now up, is a restart",
			before: map[string]dockercli.ServiceState{"web": state("c1", "i1", false)},
			after:  map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			want:   StatusRestarted,
		},
		{
			name:   "same container, already running, stays unchanged",
			before: map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			after:  map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			want:   StatusUnchanged,
		},
		{
			name: "rollup takes the most severe service outcome",
			before: map[string]dockercli.ServiceState{
				"web": state("c1", "i1", true),
				"db":  state("d1", "di1", true),
			},
			after: map[string]dockercli.ServiceState{
				"web": state("c1", "i1", true),  // unchanged
				"db":  state("d2", "di2", true), // updated
			},
			want: StatusUpdated,
		},
		{
			name:   "no after snapshot at all is unchanged",
			before: map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			after:  map[string]dockercli.ServiceState{},
			want:   StatusUnchanged,
		},
		{
			name:   "a service with no prior state is treated as recreated, not updated",
			before: map[string]dockercli.ServiceState{},
			after:  map[string]dockercli.ServiceState{"web": state("c1", "i1", true)},
			want:   StatusRecreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.before, tt.after)
			if got != tt.want {
				t.Errorf("Classify() = %s, want %s", got, tt.want)
			}
		})
	}
}
