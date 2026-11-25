package cli

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	hm "github.com/hiberus-magento/hiberus-dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Where each agent reads skills from, and what an earlier hm left where an agent no longer looks.
//

// TestCodexSkillsGoWhereCodexReadsThem: Codex scans .agents/skills from the working directory up to
// the repository root (https://learn.chatgpt.com/docs/build-skills.md), never .codex/skills, and
// OpenCode reads .agents/skills too (https://opencode.ai/docs/skills), so a generic target exists
// for any agent that reads it. The directory an earlier hm used is kept as a legacy one, to be
// reported and cleaned, never forgotten.
func TestCodexSkillsGoWhereCodexReadsThem(t *testing.T) {
	contents, err := fs.ReadFile(hm.Embedded(), "data/ai-platforms.json")
	if err != nil {
		t.Fatal(err)
	}

	var catalog struct {
		Platforms map[string]struct {
			Name             string   `json:"name"`
			SkillsDir        string   `json:"skills_dir"`
			LegacySkillsDirs []string `json:"legacy_skills_dirs"`
		} `json:"platforms"`
	}
	if err := json.Unmarshal(contents, &catalog); err != nil {
		t.Fatal(err)
	}

	codex := catalog.Platforms["codex"]
	if codex.SkillsDir != ".agents/skills" {
		t.Errorf("codex skills_dir = %q, want .agents/skills", codex.SkillsDir)
	}

	if strings.Contains(codex.Name, "GitHub") {
		t.Errorf("codex is named %q: Codex is OpenAI's, not GitHub's", codex.Name)
	}

	if len(codex.LegacySkillsDirs) != 1 || codex.LegacySkillsDirs[0] != ".codex/skills" {
		t.Errorf("codex legacy_skills_dirs = %v, want [.codex/skills]", codex.LegacySkillsDirs)
	}

	if generic := catalog.Platforms["generic"]; generic.SkillsDir != ".agents/skills" {
		t.Errorf("generic skills_dir = %q, want .agents/skills", generic.SkillsDir)
	}

	// The other platforms keep their directories
	for platform, want := range map[string]string{
		"claude": ".claude/skills", "cursor": ".cursor/skills", "copilot": ".github/skills",
		"gemini": ".gemini/skills", "opencode": ".opencode/skills",
	} {
		if got := catalog.Platforms[platform].SkillsDir; got != want {
			t.Errorf("%s skills_dir = %q, want %q", platform, got, want)
		}
	}
}

// legacyCodex is a project an earlier hm installed Codex skills into, in .codex/skills: the catalog
// knows Codex reads .agents/skills now, and the registration still holds what went to the old place.
func legacyCodex(t *testing.T) *aiFixture {
	t.Helper()

	f := aiTooling(t)
	plantFile(t, filepath.Join(f.tool, "data", "ai-platforms.json"), `{"platforms": {
		"claude": {"name": "Claude Code", "description": "Claude Code (CLI, Desktop, Web)",
		           "skills_dir": ".claude/skills", "agents_dir": ".claude/agents"},
		"codex": {"name": "OpenAI Codex", "description": "OpenAI Codex",
		          "skills_dir": ".agents/skills", "agents_dir": ".codex/agents",
		          "legacy_skills_dirs": [".codex/skills"]}
	}}`, 0o644)

	plantFile(t, f.path(".codex/skills/dockergento-environment/SKILL.md"), "# environment\n", 0o644)
	plantFile(t, f.path(".codex/skills/my-own/SKILL.md"), "# mine\n", 0o644)
	plantFile(t, f.path(core.AIRegistrationFile), `{
  "skills": {
    ".codex/skills/dockergento-environment": {"checksum": "x", "installed": "2026-01-01 00:00:00",
      "origin": "dockergento", "url": "", "branch": "", "version": "1.5.0"}
  },
  "agents": {}
}
`, 0o644)

	return f
}

const legacyReport = "Codex no longer reads .codex/skills: 1 item(s) an earlier hm installed there"

func TestAIPullReportsALegacyCodexInstallAndLeavesIt(t *testing.T) {
	f := legacyCodex(t)
	f.configure(t, `{"resources": ["skills"], "platforms": ["codex"], "types": [], "custom_repositories": []}`)

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-pull")
	if code != exitOK {
		t.Fatalf("ai-pull = %d\nstderr: %s", code, stderr)
	}

	if !f.exists(".agents/skills/dockergento-environment") {
		t.Error("the tool's skill was not installed in .agents/skills")
	}

	for _, want := range []string{legacyReport, ".codex/skills/dockergento-environment", "hm ai-reset", "hm ai-pull"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}

	// Reported, never removed: neither what hm installed nor what somebody made
	for _, name := range []string{".codex/skills/dockergento-environment", ".codex/skills/my-own"} {
		if !f.exists(name) {
			t.Errorf("%s was removed", name)
		}
	}

	if strings.Contains(stdout, legacyReport) {
		t.Errorf("the report went to stdout, where ai-pull prints nothing of its own:\n%s", stdout)
	}
}

func TestAIInitReportsALegacyCodexInstallAndLeavesIt(t *testing.T) {
	f := legacyCodex(t)

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-init", "--platforms=codex", "--resources=skills")
	if code != exitOK {
		t.Fatalf("ai-init = %d\nstderr: %s", code, stderr)
	}

	// ai-init prints its messages on stdout in text, as print_info did
	for _, want := range []string{legacyReport, ".codex/skills/dockergento-environment", "hm ai-reset"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}

	if !f.exists(".codex/skills/dockergento-environment") {
		t.Error("the legacy install was removed")
	}
}

// TestAIPullSaysNothingWithoutALegacyInstall: the previous behaviour, for a project that never had one.
func TestAIPullSaysNothingWithoutALegacyInstall(t *testing.T) {
	f := aiTooling(t)
	f.configure(t, pulledConfiguration)

	_, stderr, code := f.run(t, "", "--no-json", "ai-pull")
	if code != exitOK {
		t.Fatalf("ai-pull = %d\nstderr: %s", code, stderr)
	}

	if strings.Contains(stderr, "no longer reads") {
		t.Errorf("stderr reports a legacy install that is not there:\n%s", stderr)
	}
}

// TestAIResetCleansALegacyCodexInstall: the command the report gives has to work, so what hm put in
// a directory it used to use is still its own to remove — and only that.
func TestAIResetCleansALegacyCodexInstall(t *testing.T) {
	f := legacyCodex(t)
	f.configure(t, `{"resources": ["skills"], "platforms": ["codex"], "types": [], "custom_repositories": []}`)

	_, stderr, code := f.run(t, "", "--no-json", "ai-reset", "--confirm")
	if code != exitOK {
		t.Fatalf("ai-reset = %d\nstderr: %s", code, stderr)
	}

	if f.exists(".codex/skills/dockergento-environment") {
		t.Error("the legacy install is still there")
	}

	if !f.exists(".codex/skills/my-own") {
		t.Error("a skill hm never installed was removed")
	}

	if strings.Contains(stderr, "Refusing") {
		t.Errorf("the legacy entry was refused:\n%s", stderr)
	}
}
