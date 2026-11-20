package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// update replaces this binary with the newest release. What it downloads and replaces is in
// dockergento/release_test.go; here, what the command line asks of it and how the answer reads.
//

// stamped sets the version this binary says it was built as, for one test.
func stamped(t *testing.T, version string) {
	t.Helper()

	original := Version
	Version = version
	t.Cleanup(func() { Version = original })
}

func TestUpdateAsksForTheRunningVersionsSuccessor(t *testing.T) {
	fake := answering(t)
	stamped(t, "2.0.0")
	t.Setenv("NO_COLOR", "1")
	fake.updated = core.Updated{Updated: true, From: "2.0.0", To: "2.1.0", Latest: "2.1.0", Path: "/usr/local/bin/hm"}

	var stdout bytes.Buffer
	if code := update(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("update = %d, want exitOK", code)
	}

	if len(fake.calls) != 1 || fake.calls[0].Method != "Update" {
		t.Fatalf("asked the engine %v, want one Update", fake.calls)
	}

	if fake.updateOptions != (core.UpdateOptions{Running: "2.0.0"}) {
		t.Fatalf("asked %+v, want the running version and final releases only", fake.updateOptions)
	}

	want := "Hiberus Dockergento updated from 2.0.0 to 2.1.0\n" +
		"  installed    /usr/local/bin/hm\n" +
		"  what changed https://github.com/hiberus-magento/hiberus-dockergento/releases/tag/2.1.0\n"
	if stdout.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", stdout.String(), want)
	}
}

func TestUpdatePreAsksForPreReleasesToo(t *testing.T) {
	fake := answering(t)
	stamped(t, "2.0.0")

	update([]string{"--pre"}, io.Discard, io.Discard, true)

	if !fake.updateOptions.PreReleases {
		t.Fatalf("asked %+v, want pre-releases", fake.updateOptions)
	}
}

func TestAnUpToDateBinarySaysSo(t *testing.T) {
	fake := answering(t)
	stamped(t, "2.1.0-rc.1")
	t.Setenv("NO_COLOR", "1")
	fake.updated = core.Updated{From: "2.1.0-rc.1", To: "2.1.0-rc.1", Latest: "2.0.0"}

	var stdout bytes.Buffer
	if code := update(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("update = %d, want exitOK", code)
	}

	if want := "Hiberus Dockergento 2.1.0-rc.1 is up to date\n  The newest release is 2.0.0\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestAnUpdateAsJSON(t *testing.T) {
	fake := answering(t)
	fake.updated = core.Updated{From: "2.0.0", To: "2.0.0", Latest: "2.0.0", Path: "/x/hm"}

	var stdout bytes.Buffer
	update(nil, &stdout, io.Discard, true)

	var document struct {
		Command string
		Data    core.Updated
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}

	if document.Command != "update" || document.Data != fake.updated {
		t.Fatalf("document = %+v", document)
	}
}

// TestUpdateRefusesAnUnknownArgument: --pre is its only option.
func TestUpdateRefusesAnUnknownArgument(t *testing.T) {
	fake := answering(t)

	if code := update([]string{"--force"}, io.Discard, io.Discard, true); code != exitUsage {
		t.Fatalf("update --force = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked the engine %v, want nothing", fake.calls)
	}
}

// TestAnUpdateRefusalKeepsItsCode: a binary built from a checkout is refused with 6.
func TestAnUpdateRefusalKeepsItsCode(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.Refusal{Kind: "built_from_source", Code: 6, Message: "devel"}}}

	var stderr bytes.Buffer
	if code := update(nil, io.Discard, &stderr, true); code != 6 {
		t.Fatalf("update of a devel build = %d, want 6", code)
	}

	if !bytes.Contains(stderr.Bytes(), []byte(`"type": "built_from_source"`)) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}
