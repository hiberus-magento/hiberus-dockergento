package dockergento

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Updating the tool is pulling its own installation, which is a git checkout, and regenerating the
// shell completion from what was pulled. Never the real installation here: an origin and a clone
// of it in temporary directories, with a generate_completion.sh that leaves a mark.
//

func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	command := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// installation is an origin with one commit, a Go version (it has cmd/hm), and a clone of it that
// is the "installation".
func installation(t *testing.T) (origin, installed string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	lab := t.TempDir()
	origin = filepath.Join(lab, "origin")
	installed = filepath.Join(lab, "installed")

	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}

	// What makes a checkout a version of the Go tool, which is all hm switch moves between
	if err := os.MkdirAll(filepath.Join(origin, "cmd", "hm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "cmd", "hm", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	completion := "#!/bin/sh\ntouch \"$(dirname \"$0\")/completion-generated\"\n"
	if err := os.WriteFile(filepath.Join(origin, "generate_completion.sh"), []byte(completion), 0o755); err != nil {
		t.Fatal(err)
	}

	git(t, origin, "init", "-q", "-b", "main", ".")
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-qm", "first")
	git(t, lab, "clone", "-q", origin, installed)

	return origin, installed
}

func TestUpdatePullsTheInstallationAndRegeneratesTheCompletion(t *testing.T) {
	origin, installed := installation(t)

	if err := os.WriteFile(filepath.Join(origin, "NEW"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-qm", "second")

	if err := New(Options{Root: installed}).Update(); err != nil {
		t.Fatalf("Update = %v, want no error", err)
	}

	if _, err := os.Stat(filepath.Join(installed, "NEW")); err != nil {
		t.Errorf("what origin has was not pulled: %v", err)
	}

	if _, err := os.Stat(filepath.Join(installed, "completion-generated")); err != nil {
		t.Errorf("the completion was not regenerated: %v", err)
	}
}

func TestAnInstallationThatIsNotACheckoutCannotBeUpdated(t *testing.T) {
	err := New(Options{Root: t.TempDir()}).Update()

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "not_a_git_installation" || refusal.Code != 1 {
		t.Fatalf("Update outside a checkout = %v, want not_a_git_installation with code 1", err)
	}
}

// TestAPinnedInstallationHasNothingToUpdate: a checkout of a tag is a version somebody chose.
func TestAPinnedInstallationHasNothingToUpdate(t *testing.T) {
	_, installed := installation(t)
	git(t, installed, "tag", "1.2.3")
	git(t, installed, "checkout", "-q", "1.2.3")

	err := New(Options{Root: installed}).Update()

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "detached_installation" || refusal.Code != 6 {
		t.Fatalf("Update of a pinned installation = %v, want detached_installation with code 6", err)
	}
}

// TestAPullThatFailsIsSaid: an origin that is gone is a failed update, not a silent success.
func TestAPullThatFailsIsSaid(t *testing.T) {
	origin, installed := installation(t)

	if err := os.RemoveAll(origin); err != nil {
		t.Fatal(err)
	}

	err := New(Options{Root: installed}).Update()

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "update_failed" || refusal.Code != 1 {
		t.Fatalf("Update with no origin = %v, want update_failed with code 1", err)
	}

	if _, err := os.Stat(filepath.Join(installed, "completion-generated")); err == nil {
		t.Error("the completion was regenerated after a failed pull")
	}
}

// TestACompletionThatFailsSaysThePullHappened: by then the installation has moved, and a bare
// error would leave somebody believing it had not.
func TestACompletionThatFailsSaysThePullHappened(t *testing.T) {
	origin, installed := installation(t)

	if err := os.WriteFile(filepath.Join(origin, "generate_completion.sh"), []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-qm", "broken completion")

	err := New(Options{Root: installed}).Update()

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "completion_failed" {
		t.Fatalf("Update with a failing completion = %v, want completion_failed", err)
	}

	if !strings.Contains(refusal.Message, "updated") {
		t.Errorf("message = %q, want it to say the installation was updated", refusal.Message)
	}
}
