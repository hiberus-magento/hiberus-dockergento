package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The previews --dry-run prints: worked out from what is there, and touching none of it.
//

func TestADownPreviewNamesThisProjectsContainersAndOnlyWithVolumesItsVolumes(t *testing.T) {
	operator := Operator{Engine: &engine{containers: []core.Container{
		{ID: "2", Name: "tienda-phpfpm-1", Project: "tienda", ComposeService: "phpfpm", Running: true},
		{ID: "1", Name: "tienda-db-1", Project: "tienda", ComposeService: "db"},
		{ID: "3", Name: "otra-db-1", Project: "otra", ComposeService: "db", Running: true},
	}}}
	project := core.Project{Name: "tienda", Root: "/code/tienda"}
	present := []string{"tienda_dbdata"}
	networks := []string{"tienda_default", "tienda_back"}

	got, err := operator.DownPreview(project, core.DownOptions{Volumes: true}, present, networks)
	if err != nil {
		t.Fatalf("DownPreview(-v) = %v, want no error", err)
	}

	want := core.DownPreview{Containers: []string{"tienda-db-1", "tienda-phpfpm-1"}, Volumes: []string{"tienda_dbdata"},
		Networks: []string{"tienda_back", "tienda_default"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("DownPreview(-v) (-want +got):\n%s", diff)
	}

	got, err = operator.DownPreview(project, core.DownOptions{}, present, networks)
	if err != nil {
		t.Fatalf("DownPreview() = %v, want no error", err)
	}

	if len(got.Volumes) != 0 || got.Volumes == nil {
		t.Fatalf("DownPreview() volumes = %#v, want an empty list: without -v they stay", got.Volumes)
	}

	// The networks go with or without -v: a down always removes them
	if strings.Join(got.Networks, " ") != "tienda_back tienda_default" {
		t.Fatalf("DownPreview() networks = %#v, want both, sorted", got.Networks)
	}
}

// With no network of its own there (nil from the caller), the list is still a list.
func TestADownPreviewWithNoNetworksListsNone(t *testing.T) {
	got, err := Operator{Engine: &engine{}}.DownPreview(core.Project{Name: "tienda", Root: "/code/tienda"},
		core.DownOptions{}, nil, nil)
	if err != nil || got.Networks == nil || len(got.Networks) != 0 {
		t.Fatalf("DownPreview networks = %#v, %v, want an empty list", got.Networks, err)
	}
}

// The preview answers what the down would: from a worktree with no environment of its own, a
// refusal, not a list of the main checkout's containers.
func TestADownPreviewIsRefusedWhereTheDownWouldBe(t *testing.T) {
	operator := Operator{Engine: &engine{}, Binary: "hm"}

	_, err := operator.DownPreview(core.Project{Name: "tienda", Root: "/code/tienda", InWorktree: true},
		core.DownOptions{Volumes: true}, nil, nil)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "blocked_in_worktree" {
		t.Fatalf("DownPreview from an unregistered worktree = %v, want blocked_in_worktree", err)
	}
}

func TestARestorePreviewNamesTheCopyAndTheDatabaseAndReplacesNothing(t *testing.T) {
	corredor := &runner{answer: "CREATE TABLE pedidos (id INT);\n"}
	snapshots, project := copias(t, corredor)

	if _, err := snapshots.Take(project, "antes", false); err != nil {
		t.Fatal(err)
	}

	corredor.answer, corredor.command, corredor.fed = "magento", nil, ""

	got, err := snapshots.Preview(project, "antes")
	if err != nil {
		t.Fatalf("Preview = %v, want no error", err)
	}

	if got.Snapshot.Name != "antes" || got.Snapshot.Size == "" || got.Database != "magento" {
		t.Fatalf("Preview = %+v, want the copy and the database magento", got)
	}

	// One read, of the name: nothing dropped, nothing fed
	if corredor.fed != "" || strings.Contains(strings.Join(corredor.command, " "), "DROP") {
		t.Fatalf("Preview ran %q and fed %q, want only the name read", corredor.command, corredor.fed)
	}

	if _, err := snapshots.Preview(project, "nunca"); err == nil {
		t.Fatal("Preview(a copy that is not there) = nil, want the refusal a restore gives")
	}
}
