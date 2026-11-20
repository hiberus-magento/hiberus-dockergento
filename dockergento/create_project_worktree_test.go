package dockergento

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// create-project from a git worktree with no environment of its own is refused, as setup is and for
// the same reason: it writes the committed properties and the compose files the main checkout
// runs on. The fixtures are setup_worktree_test.go's.
//

func TestCreateProjectIsRefusedFromAWorktreeWithNoEnvironmentOfItsOwn(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")
	t.Setenv("HOME", t.TempDir())
	_, worktree := worktreeOfAProject(t)

	properties := filepath.Join(worktree, "config", "docker", "properties.json")
	before, err := os.ReadFile(properties)
	if err != nil {
		t.Fatal(err)
	}

	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}, StateDir: t.TempDir()})
	err = engine.CreateProject(worktree, core.CreateProjectOptions{
		ProjectName: "other", Domain: "other.local", Edition: "community", Version: "2.4.7", UseDefault: true,
	}, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "blocked_in_worktree" || refusal.Code != 6 {
		t.Fatalf("CreateProject from a worktree = %v, want a blocked_in_worktree refusal with code 6", err)
	}

	after, err := os.ReadFile(properties)
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != string(before) {
		t.Errorf("the committed properties changed:\nbefore %s\nafter  %s", before, after)
	}
}

// TestCreateProjectFromAWorktreeIsAllowedWhenForced: past the worktree refusal it goes on to the
// first thing create-project checks, which with a tool installation that has no table of releases
// is the version.
func TestCreateProjectFromAWorktreeIsAllowedWhenForced(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")
	t.Setenv("HOME", t.TempDir())
	_, worktree := worktreeOfAProject(t)

	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}, StateDir: t.TempDir(), Forced: true})
	err := engine.CreateProject(worktree, core.CreateProjectOptions{
		ProjectName: "other", Domain: "other.local", Edition: "community", Version: "2.4.7", UseDefault: true,
	}, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "unsupported_version" {
		t.Fatalf("CreateProject forced from a worktree = %v, want it past the worktree refusal, stopped by unsupported_version", err)
	}
}
