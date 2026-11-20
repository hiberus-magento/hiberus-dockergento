package toolinfo

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

//
// Whether this copy of the data has been anonymised.
//
// Unknown is the honest answer for data nobody has touched, and it is never treated as safe. The
// record lives outside the checkout, so it never travels with a commit to somebody else's copy.
//

func TestAnUntouchedProjectIsUnknownWithNoDate(t *testing.T) {
	state, at := State{Dir: t.TempDir()}.Anonymisation("tienda")

	if state != "unknown" || at != "" {
		t.Fatalf("Anonymisation = (%q, %q), want (unknown, no date)", state, at)
	}
}

func TestARecordedAnonymisationIsYesWithItsDate(t *testing.T) {
	state := State{Dir: t.TempDir()}

	if err := state.Record("tienda", "2026-11-20T10:00:00Z"); err != nil {
		t.Fatalf("Record = %v, want no error", err)
	}

	answer, at := state.Anonymisation("tienda")
	if answer != "yes" || at != "2026-11-20T10:00:00Z" {
		t.Fatalf("Anonymisation = (%q, %q), want (yes, the recorded date)", answer, at)
	}
}

func TestTheRecordLivesUnderTheStateDirectoryOfHome(t *testing.T) {
	// With no directory given it goes under the user's home, which is outside every checkout
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := (State{}).Record("tienda", "2026-11-20T10:00:00Z"); err != nil {
		t.Fatalf("Record = %v, want no error", err)
	}

	contents, err := os.ReadFile(filepath.Join(home, ".hm", "state", "tienda.json"))
	if err != nil {
		t.Fatalf("reading <home>/.hm/state/tienda.json = %v, want the record there", err)
	}

	if !strings.Contains(string(contents), "2026-11-20T10:00:00Z") {
		t.Fatalf("record = %q, want the date in it", contents)
	}

	if answer, _ := (State{}).Anonymisation("tienda"); answer != "yes" {
		t.Fatalf("Anonymisation = %q, want yes read back from the same place", answer)
	}
}

func TestClearingMakesItUnknownAgain(t *testing.T) {
	state := State{Dir: t.TempDir()}

	if err := state.Record("tienda", "2026-11-20T10:00:00Z"); err != nil {
		t.Fatalf("Record = %v, want no error", err)
	}

	if err := state.Clear("tienda"); err != nil {
		t.Fatalf("Clear = %v, want no error", err)
	}

	if answer, at := state.Anonymisation("tienda"); answer != "unknown" || at != "" {
		t.Fatalf("Anonymisation after Clear = (%q, %q), want (unknown, no date)", answer, at)
	}
}

func TestClearingWhatWasNeverRecordedIsNotAnError(t *testing.T) {
	// Every import clears it, and most projects were never anonymised
	if err := (State{Dir: t.TempDir()}).Clear("tienda"); err != nil {
		t.Fatalf("Clear(never recorded) = %v, want no error", err)
	}
}

func TestOneProjectsStateIsNotAnothers(t *testing.T) {
	state := State{Dir: t.TempDir()}

	if err := state.Record("tienda", "2026-11-20T10:00:00Z"); err != nil {
		t.Fatalf("Record = %v, want no error", err)
	}

	if answer, _ := state.Anonymisation("otra-tienda"); answer != "unknown" {
		t.Fatalf("Anonymisation(otra-tienda) = %q, want unknown", answer)
	}

	if err := state.Clear("otra-tienda"); err != nil {
		t.Fatalf("Clear(otra-tienda) = %v, want no error", err)
	}

	if answer, _ := state.Anonymisation("tienda"); answer != "yes" {
		t.Fatalf("Anonymisation(tienda) after clearing another = %q, want yes", answer)
	}
}

// The state file is the project's, not only the anonymisation's: anonymisation.sh merged its key
// into whatever else was there and deleted only that key. Recording or clearing keeps the rest.
func TestRecordingAndClearingKeepTheOtherKeys(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tienda.json")
	if err := os.WriteFile(file, []byte(`{"other":"kept","count":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	state := State{Dir: dir}

	if err := state.Record("tienda", "2026-11-20T10:00:00Z"); err != nil {
		t.Fatalf("Record = %v, want no error", err)
	}
	if got := readJSON(t, file); got["other"] != "kept" || got["count"] != float64(3) || got["anonymised_at"] != "2026-11-20T10:00:00Z" {
		t.Fatalf("after Record = %v, want the other keys kept and the date added", got)
	}

	if err := state.Clear("tienda"); err != nil {
		t.Fatalf("Clear = %v, want no error", err)
	}
	got := readJSON(t, file)
	if _, still := got["anonymised_at"]; still || got["other"] != "kept" || got["count"] != float64(3) {
		t.Fatalf("after Clear = %v, want only anonymised_at gone", got)
	}
	if answer, _ := state.Anonymisation("tienda"); answer != "unknown" {
		t.Fatalf("Anonymisation after Clear = %q, want unknown", answer)
	}

	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".*")); len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func readJSON(t *testing.T, file string) map[string]any {
	t.Helper()

	contents, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatalf("%s is not JSON: %v\n%s", file, err, contents)
	}

	return document
}

// A release binary's version is its stamp, even from a directory git knows nothing about.
func TestAStampedVersionIsTheVersionWithoutGit(t *testing.T) {
	if got := (Reader{Root: t.TempDir(), Stamped: "2.0.0"}).Version(); got != "2.0.0" {
		t.Fatalf("Version = %q, want the stamp", got)
	}
}

// A source build past its nearest tag is described with the commits past it, as `hm version`
// describes it: rounded down to the tag, describe and the hm.version label named an older release
// than the one running.
func TestASourceBuildPastATagIsNotRoundedDownToIt(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t",
			"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}

	run("init", "-q")
	run("commit", "-q", "--allow-empty", "-m", "one")
	run("tag", "2.0.0")
	run("commit", "-q", "--allow-empty", "-m", "two")

	got := (Reader{Root: dir}).Version()
	if !strings.HasPrefix(got, "2.0.0-1-g") {
		t.Fatalf("Version = %q, want 2.0.0-1-g<hash>", got)
	}
}
