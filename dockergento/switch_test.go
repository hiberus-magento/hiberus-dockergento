package dockergento

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Switching moves this tool's own installation, a git checkout, to a tag or to one of origin's
// branches. Never the real installation here: the origin and clone installation(t) builds, with
// tags and branches added to the origin.
//

func output(t *testing.T, dir string, args ...string) string {
	t.Helper()

	result, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}

	return strings.TrimSpace(string(result))
}

// versioned is an installation whose origin has three versions, one of them a release candidate,
// a tag that is not a version, and a second branch.
func versioned(t *testing.T) (origin, installed string) {
	t.Helper()

	origin, installed = installation(t)

	for _, tag := range []string{"1.2.0", "1.10.0", "2.0.0-rc.1", "nightly"} {
		if err := os.WriteFile(filepath.Join(origin, "VERSION"), []byte(tag), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, origin, "add", "-A")
		git(t, origin, "commit", "-qm", tag)
		git(t, origin, "tag", tag)
	}

	git(t, origin, "branch", "release/2.0.0")

	return origin, installed
}

func refusalOf(t *testing.T, err error) core.Refusal {
	t.Helper()

	var refusal core.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want a refusal", err)
	}

	return refusal
}

func TestVersionsOutsideACheckoutAreRefused(t *testing.T) {
	_, err := New(Options{Root: t.TempDir()}).Versions()

	refusal := refusalOf(t, err)
	if refusal.Kind != "not_a_git_installation" || refusal.Code != 1 ||
		refusal.Message != "This is not a git installation of Hiberus Dockergento" ||
		refusal.Hint != "Reinstall with the installer to be able to switch versions" {
		t.Fatalf("refusal = %+v, want switch.sh's not_a_git_installation", refusal)
	}
}

// TestVersionsListsWhatOriginPublished: fetched first, newest first, only what looks like a
// version, each saying whether it is a pre-release and whether it is the one installed.
func TestVersionsListsWhatOriginPublished(t *testing.T) {
	_, installed := versioned(t)

	versions, err := New(Options{Root: installed}).Versions()
	if err != nil {
		t.Fatalf("Versions = %v", err)
	}

	want := core.Versions{
		Current: "main",
		Versions: []core.Version{
			{Name: "2.0.0-rc.1", PreRelease: true},
			{Name: "1.10.0"},
			{Name: "1.2.0"},
		},
		Branches: []string{"main", "release/2.0.0"},
	}

	// origin/HEAD is reported by git as "origin" or "origin/HEAD" depending on its version; the
	// shell's filtering keeps the first, and so does this
	branches := []string{}
	for _, branch := range versions.Branches {
		if branch != "origin" {
			branches = append(branches, branch)
		}
	}
	versions.Branches = branches

	if !reflect.DeepEqual(versions, want) {
		t.Fatalf("Versions =\n%+v\nwant\n%+v", versions, want)
	}
}

// TestADetachedInstallationIsOnItsTag: a tag checked out is reported by the tag, and marked.
func TestADetachedInstallationIsOnItsTag(t *testing.T) {
	_, installed := versioned(t)
	git(t, installed, "fetch", "-q", "--tags")
	git(t, installed, "checkout", "-q", "1.10.0")

	versions, err := New(Options{Root: installed}).Versions()
	if err != nil {
		t.Fatalf("Versions = %v", err)
	}

	if versions.Current != "1.10.0" {
		t.Fatalf("current = %q, want the tag", versions.Current)
	}

	for _, version := range versions.Versions {
		if version.Installed != (version.Name == "1.10.0") {
			t.Fatalf("%+v has the wrong installed mark", version)
		}
	}
}

func TestSwitchingToAVersionDetachesAtItsTag(t *testing.T) {
	_, installed := versioned(t)

	switched, err := New(Options{Root: installed}).Switch("1.10.0")
	if err != nil {
		t.Fatalf("Switch = %v", err)
	}

	if switched != (core.Switched{Reference: "1.10.0", Version: "1.10.0"}) {
		t.Fatalf("switched = %+v", switched)
	}

	if head := output(t, installed, "rev-parse", "--abbrev-ref", "HEAD"); head != "HEAD" {
		t.Fatalf("HEAD = %q, want detached", head)
	}

	if _, err := os.Stat(filepath.Join(installed, "completion-generated")); err != nil {
		t.Errorf("the completion was not regenerated: %v", err)
	}
}

// TestSwitchingToABranchTracksIt: a local branch, so that `hm update` keeps working on it.
func TestSwitchingToABranchTracksIt(t *testing.T) {
	_, installed := versioned(t)

	switched, err := New(Options{Root: installed}).Switch("release/2.0.0")
	if err != nil {
		t.Fatalf("Switch = %v", err)
	}

	if switched.Reference != "release/2.0.0" || switched.Version != "nightly" {
		// The branch is where nightly is, with no commits after it
		t.Fatalf("switched = %+v", switched)
	}

	if upstream := output(t, installed, "rev-parse", "--abbrev-ref", "release/2.0.0@{upstream}"); upstream != "origin/release/2.0.0" {
		t.Fatalf("upstream = %q, want origin's branch", upstream)
	}
}

func TestAnUnknownReferenceIsRefusedAndNothingMoves(t *testing.T) {
	_, installed := versioned(t)

	_, err := New(Options{Root: installed, Binary: "hm"}).Switch("9.9.9")

	refusal := refusalOf(t, err)
	if refusal.Kind != "unknown_reference" || refusal.Code != 2 ||
		refusal.Message != "There is no version or branch called '9.9.9'" || refusal.Hint != "hm switch --list" {
		t.Fatalf("refusal = %+v", refusal)
	}

	if head := output(t, installed, "rev-parse", "--abbrev-ref", "HEAD"); head != "main" {
		t.Fatalf("HEAD = %q, want it where it was", head)
	}
}

// shellVersion adds to origin a version of the shell tool: a commit without cmd/hm, tagged 1.5.1,
// on a branch of its own called shell.
func shellVersion(t *testing.T, origin string) {
	t.Helper()

	git(t, origin, "checkout", "-q", "-b", "shell")
	git(t, origin, "rm", "-rq", "cmd")
	git(t, origin, "commit", "-qm", "the shell tool")
	git(t, origin, "tag", "1.5.1")
	git(t, origin, "checkout", "-q", "main")
}

// TestAVersionOfTheShellToolIsRefusedAndNothingMoves: 1.x has no cmd/hm, so there is no binary
// for it and switching to it would leave a checkout the command cannot run. Said before the
// checkout moves, with how to have a 1.x elsewhere.
func TestAVersionOfTheShellToolIsRefusedAndNothingMoves(t *testing.T) {
	origin, installed := versioned(t)
	shellVersion(t, origin)

	for _, reference := range []string{"1.5.1", "shell"} {
		_, err := New(Options{Root: installed, Binary: "hm"}).Switch(reference)

		refusal := refusalOf(t, err)
		if refusal.Kind != "version_before_go" || refusal.Code != 2 ||
			refusal.Message != "'"+reference+"' is a version of the shell tool (1.x), which hm switch cannot move to: the installation was not changed" ||
			refusal.Hint != "Install it separately, in a directory of its own: git clone -b "+reference+
				" https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm-1.x, then run ~/hm-1.x/bin/run" {
			t.Fatalf("refusal for %s = %+v", reference, refusal)
		}

		if head := output(t, installed, "rev-parse", "--abbrev-ref", "HEAD"); head != "main" {
			t.Fatalf("HEAD = %q after %s, want it where it was", head, reference)
		}

		if _, err := os.Stat(filepath.Join(installed, "completion-generated")); err == nil {
			t.Fatalf("the completion was regenerated after a refused switch to %s", reference)
		}
	}
}

// TestAVersionOfTheGoToolIsSwitchedTo: a tag whose tree has cmd/hm is one hm switch moves to.
func TestAVersionOfTheGoToolIsSwitchedTo(t *testing.T) {
	origin, installed := versioned(t)
	shellVersion(t, origin)

	if _, err := New(Options{Root: installed}).Switch("2.0.0-rc.1"); err != nil {
		t.Fatalf("Switch to a Go version = %v", err)
	}

	if now := output(t, installed, "describe", "--tags"); now != "2.0.0-rc.1" {
		t.Fatalf("installation at %q, want 2.0.0-rc.1", now)
	}
}

func TestSwitchingOutsideACheckoutIsRefused(t *testing.T) {
	_, err := New(Options{Root: t.TempDir()}).Switch("1.2.0")

	if refusal := refusalOf(t, err); refusal.Kind != "not_a_git_installation" || refusal.Code != 1 {
		t.Fatalf("refusal = %+v", refusal)
	}
}

// TestUncommittedChangesStopTheSwitch: never a stash, and what is in the way is handed back.
func TestUncommittedChangesStopTheSwitch(t *testing.T) {
	_, installed := versioned(t)

	if err := os.WriteFile(filepath.Join(installed, "generate_completion.sh"), []byte("#!/bin/sh\n# mine\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := New(Options{Root: installed}).Switch("1.2.0")

	var dirty core.DirtyInstallation
	if !errors.As(err, &dirty) {
		t.Fatalf("error = %v, want a dirty installation", err)
	}

	if dirty.Directory != installed || dirty.Changes != " M generate_completion.sh" {
		t.Fatalf("dirty = %+v, want the installation and git's own line, its leading space kept", dirty)
	}

	if refusal := refusalOf(t, err); refusal.Kind != "dirty_installation" || refusal.Code != 6 ||
		refusal.Hint != "Commit or stash them in "+installed {
		t.Fatalf("refusal = %+v", refusal)
	}

	if head := output(t, installed, "rev-parse", "--abbrev-ref", "HEAD"); head != "main" {
		t.Fatalf("HEAD = %q, want it where it was", head)
	}
}

// TestUntrackedFilesDoNotStopTheSwitch: they are the user's own, and git keeps them.
func TestUntrackedFilesDoNotStopTheSwitch(t *testing.T) {
	_, installed := versioned(t)

	if err := os.WriteFile(filepath.Join(installed, "my-notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := New(Options{Root: installed}).Switch("1.2.0"); err != nil {
		t.Fatalf("Switch with an untracked file = %v, want it to switch", err)
	}
}

// TestACheckoutGitRefusesIsSaidWithGitsWords: an untracked file the target would overwrite.
func TestACheckoutGitRefusesIsSaidWithGitsWords(t *testing.T) {
	_, installed := versioned(t)

	if err := os.WriteFile(filepath.Join(installed, "VERSION"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := New(Options{Root: installed}).Switch("1.2.0")

	refusal := refusalOf(t, err)
	if refusal.Kind != "switch_failed" || refusal.Code != 1 ||
		!strings.HasPrefix(refusal.Message, "Could not switch to '1.2.0': ") ||
		!strings.Contains(refusal.Message, "VERSION") ||
		refusal.Hint != "Check the installation directory: "+installed {
		t.Fatalf("refusal = %+v", refusal)
	}
}
