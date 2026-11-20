package dockergento

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// setup from a git worktree with no environment of its own is refused, as every command that
// creates or recreates an environment is. The shell entry point refused it (bin/run's
// validate_worktree, hm_alters_environment); the binary answered setup without that guard, and from
// such a worktree it rewrote the committed config/docker/properties.json with another project's
// name before anything else could fail.
//

// worktreeOfAProject is a committed project with a detached worktree of it, and no Docker.
func worktreeOfAProject(t *testing.T) (main, worktree string) {
	t.Helper()

	return worktreeOf(t, `{"MAGENTO_DIR": "./src", "DOMAIN": "x.test", "COMPOSE_PROJECT_NAME": "tienda"}`+"\n")
}

// worktreeOf is the same, with the committed properties given.
func worktreeOf(t *testing.T, properties string) (main, worktree string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	lab := t.TempDir()
	main = filepath.Join(lab, "main")
	worktree = filepath.Join(lab, "wt")

	if err := os.MkdirAll(filepath.Join(main, "config", "docker"), 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"docker-compose.yml":            "services:\n  phpfpm:\n    image: alpine:latest\n",
		"config/docker/properties.json": properties,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(main, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, args := range [][]string{
		{"init", "-q", "."},
		{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"},
		{"worktree", "add", "-q", "--detach", worktree, "HEAD"},
	} {
		command := exec.Command("git", args...)
		command.Dir = main
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}

	return main, worktree
}

func TestSetupIsRefusedFromAWorktreeWithNoEnvironmentOfItsOwn(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")
	t.Setenv("HOME", t.TempDir())
	_, worktree := worktreeOfAProject(t)

	properties := filepath.Join(worktree, "config", "docker", "properties.json")
	before, err := os.ReadFile(properties)
	if err != nil {
		t.Fatal(err)
	}

	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}, StateDir: t.TempDir()})
	err = engine.Setup(worktree, core.SetupOptions{ProjectName: "other", Domain: "other.test", UseDefault: true}, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "blocked_in_worktree" || refusal.Code != 6 {
		t.Fatalf("Setup from a worktree = %v, want a blocked_in_worktree refusal with code 6", err)
	}

	after, err := os.ReadFile(properties)
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != string(before) {
		t.Errorf("the committed properties changed:\nbefore %s\nafter  %s", before, after)
	}
}

// TestSetupFromAWorktreeIsAllowedWhenForced: --force is the decision to do it anyway, for one
// invocation, as with every other refusal of this kind.
func TestSetupFromAWorktreeIsAllowedWhenForced(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")
	t.Setenv("HOME", t.TempDir())
	_, worktree := worktreeOfAProject(t)

	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}, StateDir: t.TempDir(), Forced: true})
	err := engine.Setup(worktree, core.SetupOptions{ProjectName: "other", Domain: "other.test", UseDefault: true}, false)

	// It goes on to the next thing setup checks, which in this fixture is the missing composer.lock
	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "no_composer_lock" {
		t.Fatalf("Setup forced from a worktree = %v, want it past the worktree refusal, stopped by no_composer_lock", err)
	}
}

// TestSetupIsRefusedFromAWorktreeWhoseProjectDoesNotResolve: the shell entry point's guard asked
// git and looked for the main checkout's files, nothing more. A worktree whose committed properties
// are malformed does not resolve, and must not slip past the refusal into the setup that rewrites
// them.
func TestSetupIsRefusedFromAWorktreeWhoseProjectDoesNotResolve(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")
	t.Setenv("HOME", t.TempDir())
	_, worktree := worktreeOf(t, "{ this is not json\n")

	properties := filepath.Join(worktree, "config", "docker", "properties.json")
	before, err := os.ReadFile(properties)
	if err != nil {
		t.Fatal(err)
	}

	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}, StateDir: t.TempDir()})
	err = engine.Setup(worktree, core.SetupOptions{ProjectName: "other", Domain: "other.test", UseDefault: true}, false)

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "blocked_in_worktree" || refusal.Code != 6 {
		t.Fatalf("Setup from a worktree that does not resolve = %v, want a blocked_in_worktree refusal", err)
	}

	after, err := os.ReadFile(properties)
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != string(before) {
		t.Errorf("the committed properties changed:\nbefore %s\nafter  %s", before, after)
	}
}
