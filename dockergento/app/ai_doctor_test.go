package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// ai-doctor against a real tree: a project, the tool beside it with its own copies, and a
// registration naming some of what is installed — one resource in each state, in the order the
// script found them (platform by platform as the properties list them, skills before agents,
// entries by name), and what is tracked but gone last.
//

func plant(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func digestOf(path string) string {
	return core.DigestPath(os.DirFS(filepath.Dir(path)), filepath.Base(path))
}

func doctorFixture(t *testing.T) AIDoctor {
	t.Helper()

	project := t.TempDir()
	tool := t.TempDir()

	plant(t, filepath.Join(tool, "data", "ai-platforms.json"), `{"platforms": {
		"claude": {"skills_dir": ".claude/skills", "agents_dir": ".claude/agents"},
		"cursor": {"skills_dir": ".cursor/skills", "agents_dir": ".cursor/agents"}
	}}`)
	plant(t, filepath.Join(project, core.AIPropertiesFile), `{"platforms": ["cursor", "claude", "nobody"]}`)

	skills := filepath.Join(project, ".claude", "skills")
	for _, name := range []string{"current", "custom", "modified", "outdated", "unknown"} {
		plant(t, filepath.Join(skills, name, "SKILL.md"), name+"\n")
	}
	plant(t, filepath.Join(skills, ".hidden", "SKILL.md"), "hidden\n")
	plant(t, filepath.Join(project, ".claude", "agents", "reviewer.md"), "# reviewer\n")
	plant(t, filepath.Join(project, ".cursor", "skills", "absolute", "SKILL.md"), "absolute\n")

	// What came with the tool: one copy the project has fallen behind, one it has kept up with
	plant(t, filepath.Join(tool, "skills", "outdated", "SKILL.md"), "newer\n")
	plant(t, filepath.Join(tool, "skills", "current", "SKILL.md"), "current\n")

	modified := digestOf(filepath.Join(skills, "modified"))
	plant(t, filepath.Join(skills, "modified", "SKILL.md"), "edited\n")

	registration := `{
		"skills": {
			".claude/skills/current": {"checksum": "` + digestOf(filepath.Join(skills, "current")) + `",
				"origin": "hiberus", "version": "1.2.0", "installed": "2026-11-19 10:20:30"},
			".claude/skills/modified": {"checksum": "` + modified + `", "origin": "hiberus"},
			".claude/skills/outdated": {"checksum": "` + digestOf(filepath.Join(skills, "outdated")) + `"},
			".claude/skills/unknown": {"checksum": "", "installed": "2025-01-02 03:04:05"},
			"` + project + `/.cursor/skills/absolute": {"checksum": "` +
		digestOf(filepath.Join(project, ".cursor", "skills", "absolute")) + `"},
			".claude/skills/gone": {"checksum": "x"}
		},
		"agents": {
			".claude/agents/reviewer.md": {"checksum": "` +
		digestOf(filepath.Join(project, ".claude", "agents", "reviewer.md")) + `"}
		}
	}`
	plant(t, filepath.Join(project, core.AIRegistrationFile), registration)

	return AIDoctor{
		Root: project,
		Dir:  project,
		Tool: os.DirFS(tool),
	}
}

func TestTheDoctorSaysWhatStateEachResourceIsIn(t *testing.T) {
	report, err := doctorFixture(t).Report()
	if err != nil {
		t.Fatal(err)
	}

	want := core.AIDoctorReport{Configured: true, Resources: []core.AIResource{
		resource("cursor", "skills", "absolute", "current", "-", "-", "-"),
		resource("claude", "skills", "current", "current", "hiberus", "1.2.0", "2026-11-19"),
		resource("claude", "skills", "custom", "custom", "-", "-", "-"),
		resource("claude", "skills", "modified", "modified", "hiberus", "-", "-"),
		resource("claude", "skills", "outdated", "outdated", "-", "-", "-"),
		resource("claude", "skills", "unknown", "unknown", "-", "-", "2025-01-02"),
		resource("claude", "agents", "reviewer.md", "current", "-", "-", "-"),
		resource("-", "skills", "gone", "missing", "-", "-", "-"),
	}}

	if diff := cmp.Diff(want, report); diff != "" {
		t.Errorf("report (-want +got):\n%s", diff)
	}
}

func TestAProjectWithoutAIPropertiesIsNotConfigured(t *testing.T) {
	doctor := doctorFixture(t)
	if err := os.Remove(filepath.Join(doctor.Root, core.AIPropertiesFile)); err != nil {
		t.Fatal(err)
	}

	report, err := doctor.Report()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(core.AIDoctorReport{Resources: []core.AIResource{}}, report); diff != "" {
		t.Errorf("report (-want +got):\n%s", diff)
	}
}

func TestACorruptRegistrationIsRefused(t *testing.T) {
	doctor := doctorFixture(t)
	plant(t, filepath.Join(doctor.Dir, core.AIRegistrationFile), `{"skills": `)

	if _, err := doctor.Report(); !errors.Is(err, core.ErrAIRegistrationCorrupt) {
		t.Fatalf("err = %v, want ErrAIRegistrationCorrupt", err)
	}
}

func TestWithoutARegistrationEverythingIsCustom(t *testing.T) {
	doctor := doctorFixture(t)
	if err := os.Remove(filepath.Join(doctor.Dir, core.AIRegistrationFile)); err != nil {
		t.Fatal(err)
	}

	report, err := doctor.Report()
	if err != nil {
		t.Fatal(err)
	}

	for _, resource := range report.Resources {
		if resource.State != core.AIStateCustom {
			t.Errorf("%s is %s without a registration, want custom", resource.Name, resource.State)
		}
	}

	if len(report.Resources) != 7 {
		t.Errorf("%d resources, want the 7 installed", len(report.Resources))
	}
}

// The registration, and what is tracked but gone, are read from where the command runs — the
// script read config/docker/ai-registration.json relative to $PWD — while the properties and the
// installed directories are the project root's.
func TestTheRegistrationIsReadFromWhereTheCommandRuns(t *testing.T) {
	doctor := doctorFixture(t)
	doctor.Dir = t.TempDir()

	report, err := doctor.Report()
	if err != nil {
		t.Fatal(err)
	}

	if !report.Configured || len(report.Resources) != 7 {
		t.Fatalf("report = %+v, want the 7 installed and nothing tracked", report)
	}
}

// TestTheDoctorReportsWhatAPlatformNoLongerReads: a skill an earlier version installed in a
// directory the platform no longer reads is legacy, whether or not the project still asks for that
// platform; what somebody else put there is not the tool's, and not reported.
func TestTheDoctorReportsWhatAPlatformNoLongerReads(t *testing.T) {
	project, tool := t.TempDir(), t.TempDir()

	plant(t, filepath.Join(tool, "data", "ai-platforms.json"), `{"platforms": {
		"claude": {"skills_dir": ".claude/skills"},
		"codex": {"name": "OpenAI Codex", "skills_dir": ".agents/skills", "legacy_skills_dirs": [".codex/skills"]}
	}}`)
	plant(t, filepath.Join(project, core.AIPropertiesFile), `{"platforms": ["claude"]}`)
	plant(t, filepath.Join(project, ".codex", "skills", "environment", "SKILL.md"), "environment\n")
	plant(t, filepath.Join(project, ".codex", "skills", "mine", "SKILL.md"), "mine\n")
	plant(t, filepath.Join(project, core.AIRegistrationFile), `{"skills": {
		".codex/skills/environment": {"checksum": "x", "origin": "dockergento", "version": "1.5.0",
			"installed": "2026-01-02 03:04:05"}
	}}`)

	report, err := AIDoctor{Root: project, Dir: project, Tool: os.DirFS(tool)}.Report()
	if err != nil {
		t.Fatal(err)
	}

	want := core.AIDoctorReport{Configured: true, Resources: []core.AIResource{
		resource("codex", "skills", "environment", "legacy", "dockergento", "1.5.0", "2026-01-02"),
	}}

	if diff := cmp.Diff(want, report); diff != "" {
		t.Errorf("report (-want +got):\n%s", diff)
	}
}

// resource is one row of the report, in the order the script printed its columns.
func resource(platform, kind, name, state, origin, version, installed string) core.AIResource {
	return core.AIResource{
		Platform: platform, Kind: kind, Name: name, State: state,
		Origin: origin, Version: version, Installed: installed,
	}
}
