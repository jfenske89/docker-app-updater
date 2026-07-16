package dockercli

import (
	"context"
	"strings"
	"testing"
)

func TestSnapshot_MergesContainersAndImagesByName(t *testing.T) {
	executor := func(ctx context.Context, name string, args []string, dir string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "ps -a"):
			return strings.Join([]string{
				`{"ID":"c1","Name":"web-1","Service":"web","State":"running"}`,
				`{"ID":"c2","Name":"db-1","Service":"db","State":"exited"}`,
			}, "\n"), nil
		case strings.Contains(joined, "compose images"):
			return strings.Join([]string{
				`{"ID":"i1","ContainerName":"web-1","Repository":"nginx","Tag":"latest"}`,
				`{"ID":"i2","ContainerName":"db-1","Repository":"postgres","Tag":"16"}`,
			}, "\n"), nil
		default:
			return "", nil
		}
	}

	states, err := Snapshot(context.Background(), executor, "/apps/whatever")
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	web, ok := states["web"]
	if !ok {
		t.Fatal("expected a \"web\" service in the snapshot")
	}
	if web.ContainerID != "c1" || web.ImageID != "i1" || !web.Running {
		t.Errorf("web state = %+v", web)
	}

	db, ok := states["db"]
	if !ok {
		t.Fatal("expected a \"db\" service in the snapshot")
	}
	if db.ContainerID != "c2" || db.ImageID != "i2" || db.Running {
		t.Errorf("db state = %+v", db)
	}
}

func TestUnmarshalJSONLines_AcceptsArrayAndLineDelimited(t *testing.T) {
	type item struct {
		ID string `json:"ID"`
	}

	var fromArray []item
	if err := unmarshalJSONLines(`[{"ID":"a"},{"ID":"b"}]`, &fromArray); err != nil {
		t.Fatalf("array form: %v", err)
	}
	if len(fromArray) != 2 {
		t.Fatalf("array form: got %d items, want 2", len(fromArray))
	}

	var fromLines []item
	if err := unmarshalJSONLines("{\"ID\":\"a\"}\n{\"ID\":\"b\"}\n", &fromLines); err != nil {
		t.Fatalf("line-delimited form: %v", err)
	}
	if len(fromLines) != 2 {
		t.Fatalf("line-delimited form: got %d items, want 2", len(fromLines))
	}

	var fromEmpty []item
	if err := unmarshalJSONLines("", &fromEmpty); err != nil {
		t.Fatalf("empty input: %v", err)
	}
	if len(fromEmpty) != 0 {
		t.Errorf("empty input: got %d items, want 0", len(fromEmpty))
	}
}
