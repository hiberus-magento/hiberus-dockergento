package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// switch installs another release in place of this binary, or lists the releases. What it
// downloads and replaces is in dockergento/release_test.go; here, what the command line asks of it
// and how the answer is printed.
//

func listed() core.Versions {
	return core.Versions{
		Current: "2.0.0",
		Versions: []core.Version{
			{Name: "2.1.0-rc.1", PreRelease: true},
			{Name: "2.0.0", Installed: true},
			{Name: "2.0.0-rc.1", PreRelease: true},
		},
	}
}

func TestSwitchWithNothingListsTheVersions(t *testing.T) {
	for _, args := range [][]string{nil, {"--list"}, {"2.0.0", "--list"}, {"--list", "--stable"}} {
		fake := answering(t)
		stamped(t, "2.0.0")

		if code := switchVersion(args, io.Discard, io.Discard, true); code != exitOK {
			t.Fatalf("switch %v = %d", args, code)
		}

		if len(fake.calls) != 1 || fake.calls[0].Method != "Versions" || fake.versionsRunning != "2.0.0" {
			t.Fatalf("switch %v asked %v (running %q), want one Versions of 2.0.0", args, fake.calls, fake.versionsRunning)
		}
	}
}

func TestSwitchStableAsksForTheNewestFinalRelease(t *testing.T) {
	fake := answering(t)
	stamped(t, "2.0.0")

	switchVersion([]string{"--stable"}, io.Discard, io.Discard, true)

	if len(fake.calls) != 1 || fake.calls[0].Method != "Switch" ||
		fake.switchOptions != (core.SwitchOptions{Running: "2.0.0", Stable: true}) {
		t.Fatalf("switch --stable asked %v %+v, want Switch stable", fake.calls, fake.switchOptions)
	}
}

// TestTheLastVersionNamedWins: as the shell's loop overwrote its target.
func TestTheLastVersionNamedWins(t *testing.T) {
	fake := answering(t)

	switchVersion([]string{"2.0.0", "--stable", "2.1.0"}, io.Discard, io.Discard, true)

	if len(fake.calls) != 1 || fake.calls[0].Key != "2.1.0" || fake.switchOptions.Stable {
		t.Fatalf("asked %v %+v, want Switch 2.1.0", fake.calls, fake.switchOptions)
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

	if document.Data.Current != "2.0.0" || len(document.Data.Versions) != 3 ||
		!document.Data.Versions[0].PreRelease || !document.Data.Versions[1].Installed {
		t.Fatalf("data = %+v", document.Data)
	}
}

// TestAnEmptyListIsAnEmptyArray: a program iterating it expects one.
func TestAnEmptyListIsAnEmptyArray(t *testing.T) {
	fake := answering(t)
	fake.versions = core.Versions{Current: "2.0.0"}

	var stdout bytes.Buffer
	switchVersion(nil, &stdout, io.Discard, true)

	if !bytes.Contains(stdout.Bytes(), []byte(`"versions": []`)) {
		t.Fatalf("stdout = %s, want an empty array", stdout.String())
	}
}

func TestTheListAsText(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.versions = listed()

	var stdout bytes.Buffer
	if code := switchVersion(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("switch = %d", code)
	}

	want := "\nVersions\n" +
		"  2.1.0-rc.1            pre-release\n" +
		"  2.0.0                 ← installed\n" +
		"  2.0.0-rc.1            pre-release\n" +
		"\nRunning 2.0.0\n" +
		"\nSwitch with hm switch <version>, or to the newest final release with hm switch --stable\n\n"

	if stdout.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", stdout.String(), want)
	}
}

func TestAnEmptyListSaysNothingIsPublished(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.versions = core.Versions{Current: "devel"}

	var stdout bytes.Buffer
	switchVersion(nil, &stdout, io.Discard, false)

	if want := "\nVersions\n  no release of 2.0 or later is published yet\n\nRunning devel\n"; !bytes.HasPrefix(stdout.Bytes(), []byte(want)) {
		t.Fatalf("stdout =\n%q\nwant it to start\n%q", stdout.String(), want)
	}
}

func TestASwitchSaysWhatIsRunningNow(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.switched = core.Switched{Reference: "2.1.0", Version: "2.1.0", From: "2.0.0", Path: "/x/hm"}

	var stdout bytes.Buffer
	if code := switchVersion([]string{"2.1.0"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("switch = %d", code)
	}

	want := "\nNow running Hiberus Dockergento 2.1.0\n" +
		"  What changed: https://github.com/hiberus-magento/hiberus-dockergento/releases/tag/2.1.0\n\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestASwitchToTheRunningVersionSaysSo(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.switched = core.Switched{Reference: "2.0.0", Version: "2.0.0", Unchanged: true}

	var stdout bytes.Buffer
	switchVersion([]string{"2.0.0"}, &stdout, io.Discard, false)

	if want := "\nAlready running Hiberus Dockergento 2.0.0\n\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestASwitchAsJSON(t *testing.T) {
	fake := answering(t)
	fake.switched = core.Switched{Reference: "2.1.0", Version: "2.1.0", From: "2.0.0", Path: "/x/hm"}

	var stdout bytes.Buffer
	switchVersion([]string{"2.1.0"}, &stdout, io.Discard, true)

	var document struct {
		Command string
		Data    core.Switched
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}

	if document.Command != "switch" || document.Data != fake.switched {
		t.Fatalf("document = %+v", document)
	}
}

func TestASwitchRefusalKeepsItsCode(t *testing.T) {
	for _, args := range [][]string{{"1.5.1"}, nil} {
		fake := answering(t)
		fake.outcomes = []outcome{{err: core.Refusal{Kind: "version_before_go", Code: exitUsage, Message: "no"}}}

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
