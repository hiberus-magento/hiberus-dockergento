package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// switch moves this tool's installation to a version or a branch, or lists what it can move to.
// The git side is in dockergento/switch_test.go; here, what the command line asks of it and how
// the answer is printed, which is switch.sh's text byte for byte.
//

func listed() core.Versions {
	return core.Versions{
		Current: "1.10.0",
		Versions: []core.Version{
			{Name: "2.0.0-rc.1", PreRelease: true},
			{Name: "1.10.0", Installed: true},
			{Name: "1.2.0"},
		},
		Branches: []string{"main", "release/2.0.0"},
	}
}

func TestSwitchWithNothingListsTheVersions(t *testing.T) {
	for _, args := range [][]string{nil, {"--list"}, {"1.2.0", "--list"}, {"--list", "--stable"}} {
		fake := answering(t)

		if code := switchVersion(args, io.Discard, io.Discard, true); code != exitOK {
			t.Fatalf("switch %v = %d", args, code)
		}

		if len(fake.calls) != 1 || fake.calls[0].Method != "Versions" {
			t.Fatalf("switch %v asked %v, want one Versions", args, fake.calls)
		}
	}
}

func TestSwitchStableGoesToMain(t *testing.T) {
	fake := answering(t)

	switchVersion([]string{"--stable"}, io.Discard, io.Discard, true)

	if len(fake.calls) != 1 || fake.calls[0].Method != "Switch" || fake.calls[0].Key != "main" {
		t.Fatalf("switch --stable asked %v, want Switch main", fake.calls)
	}
}

// TestTheLastReferenceNamedWins: as the shell's loop overwrote its target.
func TestTheLastReferenceNamedWins(t *testing.T) {
	fake := answering(t)

	switchVersion([]string{"1.2.0", "--stable", "release/2.0.0"}, io.Discard, io.Discard, true)

	if len(fake.calls) != 1 || fake.calls[0].Key != "release/2.0.0" {
		t.Fatalf("asked %v, want Switch release/2.0.0", fake.calls)
	}
}

func TestSwitchRefusesAnUnknownOption(t *testing.T) {
	fake := answering(t)

	var stderr bytes.Buffer
	if code := switchVersion([]string{"--force"}, io.Discard, &stderr, true); code != exitUsage {
		t.Fatalf("switch --force = %d, want exitUsage", code)
	}

	var document struct {
		Error struct{ Type, Message, Hint string }
	}
	if err := json.Unmarshal(stderr.Bytes(), &document); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr.String())
	}

	if document.Error.Type != "invalid_argument" || document.Error.Message != "Unknown option: --force" ||
		document.Error.Hint != "hm switch --help" {
		t.Fatalf("error = %+v", document.Error)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("asked the engine %v, want nothing", fake.calls)
	}
}

func TestTheListAsJSON(t *testing.T) {
	fake := answering(t)
	fake.versions = listed()

	var stdout bytes.Buffer
	switchVersion(nil, &stdout, io.Discard, true)

	var document struct {
		Data core.Versions
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}

	if document.Data.Current != "1.10.0" || len(document.Data.Versions) != 3 ||
		!document.Data.Versions[0].PreRelease || !document.Data.Versions[1].Installed ||
		len(document.Data.Branches) != 2 {
		t.Fatalf("data = %+v", document.Data)
	}
}

// TestAnEmptyListIsEmptyArrays: jq gave [] for nothing, and a program iterating it expects one.
func TestAnEmptyListIsEmptyArrays(t *testing.T) {
	fake := answering(t)
	fake.versions = core.Versions{Current: "main"}

	var stdout bytes.Buffer
	switchVersion(nil, &stdout, io.Discard, true)

	if !bytes.Contains(stdout.Bytes(), []byte(`"versions": []`)) || !bytes.Contains(stdout.Bytes(), []byte(`"branches": []`)) {
		t.Fatalf("stdout = %s, want empty arrays", stdout.String())
	}
}

func TestTheListAsText(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.versions = listed()
	fake.versions.Current = "main"
	fake.versions.Versions[1].Installed = false
	fake.versions.Versions[0].Installed = true

	var stdout bytes.Buffer
	if code := switchVersion(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("switch = %d", code)
	}

	// The shell decided installed by comparing with current, not by a field: here current is
	// main, and the rc is marked installed only to show how both marks print together
	want := "\nVersions\n" +
		"  2.0.0-rc.1            ← installed   pre-release\n" +
		"  1.10.0\n" +
		"  1.2.0\n" +
		"\nBranches\n" +
		"  main   ← installed\n" +
		"  release/2.0.0\n" +
		"\nSwitch with hm switch <version|branch>, or back to stable with hm switch --stable\n\n"

	if stdout.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", stdout.String(), want)
	}
}

func TestAPreReleaseIsLabelled(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.versions = core.Versions{Current: "main", Versions: []core.Version{{Name: "2.0.0-rc.1", PreRelease: true}}}

	var stdout bytes.Buffer
	switchVersion(nil, &stdout, io.Discard, false)

	if want := "\nVersions\n  2.0.0-rc.1            pre-release\n\nBranches\n"; !bytes.HasPrefix(stdout.Bytes(), []byte(want)) {
		t.Fatalf("stdout =\n%q\nwant it to start\n%q", stdout.String(), want)
	}
}

func TestASwitchSaysWhatIsRunningNow(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.switched = core.Switched{Reference: "1.10.0", Version: "1.10.0"}

	var stdout bytes.Buffer
	if code := switchVersion([]string{"1.10.0"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("switch = %d", code)
	}

	if want := "\nNow running Hiberus Dockergento 1.10.0\n  What changed: changelogs/\n\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestASwitchAsJSON(t *testing.T) {
	fake := answering(t)
	fake.switched = core.Switched{Reference: "release/2.0.0", Version: "2.0.0-rc.1-3-gabc"}

	var stdout bytes.Buffer
	switchVersion([]string{"release/2.0.0"}, &stdout, io.Discard, true)

	var document struct {
		Command string
		Data    map[string]string
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}

	if document.Data["switched_to"] != "release/2.0.0" || document.Data["version"] != "2.0.0-rc.1-3-gabc" ||
		document.Command != "switch" {
		t.Fatalf("document = %+v", document)
	}
}

// TestUncommittedChangesAreShownToAPerson: on stdout, as switch.sh printed them, and refused.
func TestUncommittedChangesAreShownToAPerson(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.outcomes = []outcome{{err: core.DirtyInstallation{Directory: "/opt/hm", Changes: " M README.md\nM  bin/run"}}}

	var stdout bytes.Buffer
	if code := switchVersion([]string{"1.2.0"}, &stdout, io.Discard, false); code != exitBlocked {
		t.Fatalf("switch over changes = %d, want exitBlocked", code)
	}

	want := "\nThe installation directory has uncommitted changes:\n\n" +
		"     M README.md\n" +
		"    M  bin/run\n" +
		"\nNothing was changed. Commit or stash them first in:\n  /opt/hm\n\n"

	if stdout.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", stdout.String(), want)
	}
}

// TestUncommittedChangesAreARefusalToAProgram: the refusal, on stderr, with its kind.
func TestUncommittedChangesAreARefusalToAProgram(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.DirtyInstallation{Directory: "/opt/hm", Changes: " M README.md"}}}

	var stdout, stderr bytes.Buffer
	if code := switchVersion([]string{"1.2.0"}, &stdout, &stderr, true); code != exitBlocked {
		t.Fatalf("switch over changes = %d, want exitBlocked", code)
	}

	if stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte(`"type": "dirty_installation"`)) ||
		!bytes.Contains(stderr.Bytes(), []byte(`Commit or stash them in /opt/hm`)) {
		t.Fatalf("stdout = %q, stderr = %s", stdout.String(), stderr.String())
	}
}

func TestASwitchRefusalKeepsItsCode(t *testing.T) {
	for _, args := range [][]string{{"9.9.9"}, nil} {
		fake := answering(t)
		fake.outcomes = []outcome{{err: core.Refusal{Kind: "unknown_reference", Code: exitUsage, Message: "no"}}}

		if code := switchVersion(args, io.Discard, io.Discard, true); code != exitUsage {
			t.Fatalf("switch %v refused with 2 = %d", args, code)
		}
	}
}

func TestTheRouterAnswersForSwitch(t *testing.T) {
	fake := answering(t)

	Run([]string{"switch", "--list"}, io.Discard, io.Discard)

	if len(fake.calls) != 1 || fake.calls[0].Method != "Versions" {
		t.Fatalf("switch fell through to the shell implementation: %v", fake.calls)
	}
}
