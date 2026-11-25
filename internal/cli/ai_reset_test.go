package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// ai-reset, in the words and streams of console/commands/ai-reset.sh: only what the registration
// names is removed, and what somebody made survives.
//

// installed is a project after a pull, with a skill and an agent of somebody's own beside it.
func installed(t *testing.T) *aiFixture {
	t.Helper()

	f := aiTooling(t)
	f.configure(t, pulledConfiguration)

	if _, stderr, code := f.run(t, "", "--no-json", "ai-pull"); code != exitOK {
		t.Fatalf("ai-pull = %d\n%s", code, stderr)
	}

	plantFile(t, f.path(".claude/skills/mine/SKILL.md"), "mine\n", 0o644)
	plantFile(t, f.path(".claude/agents/mine.md"), "mine\n", 0o644)

	return f
}

const resetList = ".claude/skills/dockergento-database\n" +
	".claude/skills/dockergento-environment\n" +
	".claude/skills/hyva-checkout\n" +
	".claude/agents/reviewer.md\n"

func TestAIResetRemovesOnlyWhatWasInstalled(t *testing.T) {
	f := installed(t)

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
	if code != exitOK {
		t.Fatalf("ai-reset = %d\n%s", code, stderr)
	}

	for name, want := range map[string]bool{
		".claude/skills/dockergento-database": false, ".claude/skills/hyva-checkout": false,
		".claude/agents/reviewer.md": false, ".claude/skills/mine/SKILL.md": true,
		".claude/agents/mine.md": true, core.AIPropertiesFile: true,
	} {
		if f.exists(name) != want {
			t.Errorf("%s exists = %v, want %v", name, !want, want)
		}
	}

	if got, want := f.read(t, core.AIRegistrationFile), "{\n  \"skills\": {},\n  \"agents\": {}\n}\n"; got != want {
		t.Errorf("registration = %q, want %q", got, want)
	}

	if mode := f.mode(t, core.AIRegistrationFile); mode != 0o600 {
		t.Errorf("registration mode = %o, want 600", mode)
	}

	if want := "\n" + resetList + "\n\n\n\n\n\n"; stdout != want {
		t.Errorf("stdout (-want +got):\n%s", cmp.Diff(want, stdout))
	}

	want := lines(
		"Loading registration data...",
		"Collecting tracked files...",
		"The following 4 items will be removed:",
		"Custom skills/agents will be preserved:",
		"  claude custom skills: 1",
		"Total custom items that will be preserved: 1",
		"Removing tracked files...",
		"  Removed directory: .claude/skills/dockergento-database",
		"  Removed directory: .claude/skills/dockergento-environment",
		"  Removed directory: .claude/skills/hyva-checkout",
		"  Removed file: .claude/agents/reviewer.md",
		"Clearing registration...",
		"AI tools reset complete!",
		"Removed 4 items",
		"Configuration file preserved: config/docker/ai-properties.json",
		"Registration file cleared: config/docker/ai-registration.json",
		"To re-download AI tools, run: hm ai-pull",
	)
	if diff := cmp.Diff(want, stderr); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}
}

func TestAIResetAsksFirstAndANoKeepsEverything(t *testing.T) {
	f := installed(t)
	t.Setenv("HM_NON_INTERACTIVE", "")
	before := f.read(t, core.AIRegistrationFile)

	stdout, stderr, code := f.run(t, "n\n", "--no-json", "ai-reset")
	if code != exitOK {
		t.Fatalf("ai-reset = %d", code)
	}

	if !strings.HasSuffix(stdout, "\nContinue with deletion? (y/N) ") {
		t.Errorf("stdout does not end with the question:\n%q", stdout)
	}

	if !strings.HasSuffix(stderr, "This action cannot be undone.\nReset cancelled\n") {
		t.Errorf("stderr does not end with the cancellation:\n%s", stderr)
	}

	if !f.exists(".claude/skills/hyva-checkout") || f.read(t, core.AIRegistrationFile) != before {
		t.Error("something changed after a no")
	}
}

func TestAIResetAsksFirstAndAYesRemoves(t *testing.T) {
	f := installed(t)
	t.Setenv("HM_NON_INTERACTIVE", "")

	if _, _, code := f.run(t, "Yes\n", "--no-json", "ai-reset"); code != exitOK {
		t.Fatalf("ai-reset = %d", code)
	}

	if f.exists(".claude/skills/hyva-checkout") || !f.exists(".claude/skills/mine") {
		t.Error("a yes did not remove exactly what was installed")
	}
}

// With nobody to ask, the question is skipped and the reset goes ahead, as the script did.
func TestAIResetWithNobodyToAskGoesAhead(t *testing.T) {
	f := installed(t)

	stdout, _, code := f.run(t, "", "--no-json", "ai-reset")
	if code != exitOK {
		t.Fatalf("ai-reset = %d", code)
	}

	if strings.Contains(stdout, "Continue with deletion?") {
		t.Error("a question was asked with nobody to answer it")
	}

	if f.exists(".claude/skills/hyva-checkout") {
		t.Error("nothing was removed")
	}
}

func TestAIResetSaysWhatWasAlreadyGone(t *testing.T) {
	f := installed(t)

	if err := os.RemoveAll(f.path(".claude/skills/hyva-checkout")); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
	if code != exitOK {
		t.Fatalf("ai-reset = %d", code)
	}

	for _, line := range []string{"  File not found (already removed?): .claude/skills/hyva-checkout\n", "Removed 3 items\n"} {
		if !strings.Contains(stderr, line) {
			t.Errorf("stderr has no %q:\n%s", line, stderr)
		}
	}
}

func TestAIResetWithNothingTrackedHasNothingToDo(t *testing.T) {
	f := aiTooling(t)

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
	if code != exitOK {
		t.Fatalf("ai-reset = %d", code)
	}

	want := lines("Loading registration data...", "No tracked AI tools found in registration", "Nothing to reset")
	if stderr != want || stdout != "" {
		t.Errorf("stdout %q, stderr (-want +got):\n%s", stdout, cmp.Diff(want, stderr))
	}

	if f.exists(core.AIRegistrationFile) {
		t.Error("a registration was written")
	}
}

func TestAIResetRefusesACorruptRegistration(t *testing.T) {
	f := installed(t)
	plantFile(t, f.path(core.AIRegistrationFile), "not json", 0o600)

	_, stderr, code := f.run(t, "", "--json", "ai-reset", "--confirm")
	if code != exitError {
		t.Fatalf("ai-reset = %d, want 1", code)
	}

	want := "Loading registration data...\n" +
		"Invalid JSON in config/docker/ai-registration.jsonRegistration file is corrupted - refusing to operate" +
		"Failed to load ai-registration.json\n" +
		"Registration file may be corrupted or missing\n" +
		"If the file is corrupted, manually delete config/docker/ai-registration.json\n"
	if diff := cmp.Diff(want, stderr); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}

	if !f.exists(".claude/skills/hyva-checkout") {
		t.Error("something was removed")
	}
}

func TestAIResetRefusesAnUnknownOption(t *testing.T) {
	f := aiTooling(t)

	stdout, _, code := f.run(t, "", "--no-json", "ai-reset", "--not-an-option")
	if code != exitUsage {
		t.Fatalf("ai-reset = %d, want 2", code)
	}

	if want := "Unknown option: --not-an-option\n" + aiResetUsage; stdout != want {
		t.Errorf("stdout (-want +got):\n%s", cmp.Diff(want, stdout))
	}
}

// A configuration that is not JSON stopped the script in the middle of its summary, in text, on
// jq's failure: before anything was removed.
func TestAIResetStopsOnAnInvalidConfigurationBeforeRemoving(t *testing.T) {
	f := installed(t)
	f.configure(t, "{bad")

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
	if code != 5 {
		t.Fatalf("ai-reset = %d, want 5", code)
	}

	if !strings.HasSuffix(stdout, "\n{}\n") || !strings.HasSuffix(stderr, "jq: parse error: Invalid numeric literal at line 1, column 8\n") {
		t.Errorf("stdout %q\nstderr %q", stdout, stderr)
	}

	if !f.exists(".claude/skills/hyva-checkout") {
		t.Error("something was removed")
	}
}

// A registration is a file anybody can edit: what it names outside the platforms' own skills and
// agents directories under the project is refused, kept in the registration, and said.
func TestAIResetRefusesWhatIsNotInsideTheToolsDirectories(t *testing.T) {
	f := installed(t)

	outside := t.TempDir()
	plantFile(t, filepath.Join(outside, "victim", "keep.txt"), "keep\n", 0o644)
	plantFile(t, filepath.Join(outside, "file.md"), "keep\n", 0o644)
	plantFile(t, f.path("config/docker/keep.txt"), "keep\n", 0o644)
	plantFile(t, f.path(".claude/skills/hyva-checkout/nested/keep.txt"), "keep\n", 0o644)

	// A platform directory that leads out of the project
	if err := os.MkdirAll(f.path(".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, f.path(".cursor/skills")); err != nil {
		t.Fatal(err)
	}

	hostile := []string{
		filepath.Join(outside, "victim"),
		"../" + filepath.Base(outside) + "/victim",
		".claude/skills/../../config/docker/keep.txt",
		"config/docker/keep.txt",
		".claude/skills",
		".claude/skills/hyva-checkout/nested",
		".cursor/skills/victim",
		".claude/agents/../../" + "config/docker/keep.txt",
	}

	registration := `{"skills": {".claude/skills/hyva-checkout": {"checksum": "x"}`
	for _, path := range hostile[:7] {
		registration += `, "` + path + `": {"checksum": "x"}`
	}

	registration += `}, "agents": {".claude/agents/reviewer.md": {"checksum": "x"}, "` + hostile[7] + `": {"checksum": "x"}}}`
	plantFile(t, f.path(core.AIRegistrationFile), registration, 0o600)

	_, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
	if code != exitError {
		t.Fatalf("ai-reset = %d, want 1\n%s", code, stderr)
	}

	for _, kept := range []string{
		filepath.Join(outside, "victim", "keep.txt"), filepath.Join(outside, "file.md"),
		f.path("config/docker/keep.txt"), f.path(".claude/skills/mine/SKILL.md"),
	} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s was removed", kept)
		}
	}

	if f.exists(".claude/skills/hyva-checkout") || f.exists(".claude/agents/reviewer.md") {
		t.Error("what was really installed was not removed")
	}

	for _, path := range hostile {
		if want := "  Refusing to remove " + path + ": not inside the project's AI tools directories\n"; !strings.Contains(stderr, want) {
			t.Errorf("stderr has no %q:\n%s", want, stderr)
		}

		if !strings.Contains(f.read(t, core.AIRegistrationFile), `"`+path+`"`) {
			t.Errorf("the registration lost %s", path)
		}
	}

	if strings.Contains(f.read(t, core.AIRegistrationFile), `".claude/skills/hyva-checkout"`) {
		t.Error("the registration still names what was removed")
	}
}

// What could not be removed stays registered, so a later reset still knows it is the tool's.
func TestAIResetKeepsTheRegistrationOfWhatItCouldNotRemove(t *testing.T) {
	f := installed(t)

	if err := os.Chmod(f.path(".claude/agents"), 0o555); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Chmod(f.path(".claude/agents"), 0o755) }) //nolint:errcheck

	_, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
	if code != exitError {
		t.Fatalf("ai-reset = %d, want 1\n%s", code, stderr)
	}

	for _, line := range []string{
		"  Failed to remove file: .claude/agents/reviewer.md\n",
		"Registration kept for 1 items that were not removed\n",
	} {
		if !strings.Contains(stderr, line) {
			t.Errorf("stderr has no %q:\n%s", line, stderr)
		}
	}

	registration := f.read(t, core.AIRegistrationFile)
	if !strings.Contains(registration, `".claude/agents/reviewer.md"`) || strings.Contains(registration, `".claude/skills/hyva-checkout"`) {
		t.Errorf("registration = %s", registration)
	}
}

// A registration whose skills or agents are not objects is refused before anything is removed.
func TestAIResetRefusesARegistrationOfTheWrongShape(t *testing.T) {
	for _, registration := range []string{
		`{"skills": [".claude/skills/hyva-checkout"], "agents": {}}`,
		`{"skills": {}, "agents": "x"}`,
		`{"skills": {".claude/skills/hyva-checkout": {}}, "agents": 3}`,
	} {
		t.Run(registration, func(t *testing.T) {
			f := installed(t)
			plantFile(t, f.path(core.AIRegistrationFile), registration, 0o600)

			_, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
			if code != exitError {
				t.Fatalf("ai-reset = %d, want 1\n%s", code, stderr)
			}

			if want := "Invalid registration JSON structure (skills and agents must be objects)\n"; !strings.Contains(stderr, want) {
				t.Errorf("stderr has no %q:\n%s", want, stderr)
			}

			if !f.exists(".claude/skills/hyva-checkout") || f.read(t, core.AIRegistrationFile) != registration {
				t.Error("something changed")
			}
		})
	}
}
