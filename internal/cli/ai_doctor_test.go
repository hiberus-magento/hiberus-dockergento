package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// ai-doctor prints what the engine found (app/ai_doctor_test.go holds how it finds it), in the
// words, columns and streams of console/commands/ai-doctor.sh.
//

// TestAIDoctorSaysHowToCleanALegacyInstall: what a platform no longer reads is reported with the
// command that cleans it, and nothing is removed.
func TestAIDoctorSaysHowToCleanALegacyInstall(t *testing.T) {
	fake := answering(t)
	fake.aiDoctor = core.AIDoctorReport{Configured: true, Resources: []core.AIResource{
		resource("codex", "skills", "dockergento-environment", "legacy", "dockergento", "1.5.0", "2026-01-02"),
	}}
	t.Setenv("NO_COLOR", "1")

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--no-json", "ai-doctor"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("ai-doctor = %d", code)
	}

	if want := "1 installed where its platform no longer reads them\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}

	if want := "  hm ai-reset && hm ai-pull\n"; !bytes.Contains(stdout.Bytes(), []byte(want)) {
		t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
	}
}

func doctorReport() core.AIDoctorReport {
	return core.AIDoctorReport{Configured: true, Resources: []core.AIResource{
		resource("claude", "skills", "magento-expert", "current", "hiberus", "1.2.0", "2026-11-19"),
		resource("claude", "skills", "mine", "custom", "-", "-", "-"),
		resource("claude", "skills", "edited", "modified", "hiberus", "-", "2026-01-02"),
		resource("claude", "skills", "old", "outdated", "-", "-", "-"),
		resource("claude", "agents", "reviewer.md", "unknown", "-", "-", "-"),
		resource("-", "skills", "gone", "missing", "-", "-", "-"),
	}}
}

func TestAIDoctorPrintsTheTable(t *testing.T) {
	fake := answering(t)
	fake.aiDoctor = doctorReport()
	t.Setenv("NO_COLOR", "1")

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--no-json", "ai-doctor"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("ai-doctor = %d\nstderr: %s", code, stderr.String())
	}

	row := func(platform, name, state, from, installed string) string {
		return fmt.Sprintf("  %-10s %-30s %-9s %-24s %s\n", platform, name, state, from, installed)
	}

	want := "\nAI tooling in this project\n\n" +
		row("PLATFORM", "NAME", "STATE", "FROM", "INSTALLED") +
		row("claude", "magento-expert", "current", "hiberus 1.2.0", "2026-11-19") +
		row("claude", "mine", "custom", "-", "-") +
		row("claude", "edited", "modified", "hiberus", "2026-01-02") +
		row("claude", "old", "outdated", "-", "-") +
		row("claude", "reviewer.md", "unknown", "-", "-") +
		row("-", "gone", "missing", "-", "-") +
		"\n" +
		"  hm ai-pull\n" +
		"  Those edits are lost on the next hm ai-pull.\n" +
		"  Rename them to keep them: what the tool did not install, it does not touch.\n" +
		"\n"

	if diff := cmp.Diff(want, stdout.String()); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}

	wantErr := "1 behind the version that came with this tool\n1 changed since they were installed\n"
	if diff := cmp.Diff(wantErr, stderr.String()); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]call{{Method: "AIDoctor", Dir: here()}}, fake.calls); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}
}

// The script counted lines with grep -c, so a row is counted for the word anywhere in it: a skill
// named after the state counts as one more.
func TestAIDoctorCountsTheRowsThatSayTheWord(t *testing.T) {
	fake := answering(t)
	fake.aiDoctor = core.AIDoctorReport{Configured: true, Resources: []core.AIResource{
		resource("claude", "skills", "outdated-notes", "current", "-", "-", "-"),
		resource("claude", "skills", "old", "outdated", "-", "-", "-"),
	}}
	t.Setenv("NO_COLOR", "1")

	var stderr bytes.Buffer
	if code := Run([]string{"--no-json", "ai-doctor"}, &bytes.Buffer{}, &stderr); code != exitOK {
		t.Fatalf("ai-doctor = %d", code)
	}

	if want := "2 behind the version that came with this tool\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestAIDoctorWithNothingInstalledSaysHowToInstall(t *testing.T) {
	fake := answering(t)
	fake.aiDoctor = core.AIDoctorReport{Configured: true, Resources: []core.AIResource{}}
	t.Setenv("NO_COLOR", "1")

	var stdout bytes.Buffer
	if code := Run([]string{"--no-json", "ai-doctor"}, &stdout, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("ai-doctor = %d", code)
	}

	if want := "Nothing installed yet.\n  hm ai-pull\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestAIDoctorWithoutAIToolingSaysHowToStart(t *testing.T) {
	fake := answering(t)
	fake.aiDoctor = core.AIDoctorReport{Resources: []core.AIResource{}}
	t.Setenv("NO_COLOR", "1")

	var stdout bytes.Buffer
	if code := Run([]string{"--no-json", "ai-doctor"}, &stdout, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("ai-doctor = %d", code)
	}

	if want := "This project has no AI tooling configured.\n  hm ai-init\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}

	var document bytes.Buffer
	if code := Run([]string{"--json", "ai-doctor"}, &document, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("ai-doctor --json = %d", code)
	}

	want := `{
  "schema_version": 1,
  "command": "ai-doctor",
  "ok": true,
  "data": {
    "configured": false,
    "resources": []
  }
}
`
	if diff := cmp.Diff(want, document.String()); diff != "" {
		t.Errorf("document (-want +got):\n%s", diff)
	}
}

func TestAIDoctorGivesTheResourcesAsADocument(t *testing.T) {
	fake := answering(t)
	fake.aiDoctor = core.AIDoctorReport{Configured: true, Resources: doctorReport().Resources[:1]}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "ai-doctor"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("ai-doctor --json = %d", code)
	}

	want := `{
  "schema_version": 1,
  "command": "ai-doctor",
  "ok": true,
  "data": {
    "configured": true,
    "resources": [
      {
        "platform": "claude",
        "kind": "skills",
        "name": "magento-expert",
        "state": "current",
        "origin": "hiberus",
        "version": "1.2.0",
        "installed": "2026-11-19"
      }
    ]
  }
}
`
	if diff := cmp.Diff(want, stdout.String()); diff != "" {
		t.Errorf("document (-want +got):\n%s", diff)
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing beside the document", stderr.String())
	}
}

func TestAIDoctorRefusesAnUnknownOption(t *testing.T) {
	fake := answering(t)

	var stderr bytes.Buffer
	if code := Run([]string{"--json", "ai-doctor", "--bogus"}, &bytes.Buffer{}, &stderr); code != exitUsage {
		t.Fatalf("ai-doctor --bogus = %d, want exitUsage", code)
	}

	want := `{
  "schema_version": 1,
  "command": "ai-doctor",
  "ok": false,
  "error": {
    "code": 2,
    "type": "invalid_argument",
    "message": "Unknown option: --bogus",
    "hint": "hm ai-doctor"
  }
}
`
	if diff := cmp.Diff(want, stderr.String()); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}

	if len(fake.calls) != 0 {
		t.Errorf("calls = %v, want none", fake.calls)
	}
}

// The script read the registration inside a command substitution, so in text its complaint was
// swallowed with the rest of stdout and only the exit code said anything; under --json the
// complaint went to stderr, the two messages run together as print_error and print_warning left
// them.
func TestAIDoctorRefusesACorruptRegistration(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.ErrAIRegistrationCorrupt}}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--no-json", "ai-doctor"}, &stdout, &stderr); code != exitProject {
		t.Fatalf("ai-doctor = %d, want exitProject", code)
	}

	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("text printed %q / %q, want nothing", stdout.String(), stderr.String())
	}

	fake.outcomes = []outcome{{err: core.ErrAIRegistrationCorrupt}}
	fake.calls = nil
	stdout.Reset()

	if code := Run([]string{"--json", "ai-doctor"}, &stdout, &stderr); code != exitProject {
		t.Fatalf("ai-doctor --json = %d, want exitProject", code)
	}

	want := "Invalid JSON in config/docker/ai-registration.jsonRegistration file is corrupted - refusing to operate"
	if stdout.Len() != 0 || stderr.String() != want {
		t.Errorf("--json printed %q / %q, want nothing / %q", stdout.String(), stderr.String(), want)
	}
}

func TestAIDoctorThatCannotReadSaysSo(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: errors.New("permission denied")}}

	if code := Run([]string{"--json", "ai-doctor"}, &bytes.Buffer{}, &bytes.Buffer{}); code == exitOK {
		t.Fatal("ai-doctor that could not read = 0, want a failure")
	}
}

// resource is one row of the report, in the order the script printed its columns.
func resource(platform, kind, name, state, origin, version, installed string) core.AIResource {
	return core.AIResource{
		Platform: platform, Kind: kind, Name: name, State: state,
		Origin: origin, Version: version, Installed: installed,
	}
}
