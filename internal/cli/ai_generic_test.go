package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The generic platform is .agents/skills and nothing else: it has no directory for agents, and
// ai-init says so instead of installing nothing for them in silence.
//

// genericTooling is the AI fixture whose catalog has the generic platform beside one with agents.
func genericTooling(t *testing.T) *aiFixture {
	t.Helper()

	f := aiTooling(t)
	plantFile(t, filepath.Join(f.tool, "data", "ai-platforms.json"), `{"platforms": {
		"claude": {"name": "Claude Code", "description": "Claude Code (CLI, Desktop, Web)",
		           "skills_dir": ".claude/skills", "agents_dir": ".claude/agents"},
		"generic": {"name": "Any agent reading .agents/skills", "description": "Any agent",
		            "skills_dir": ".agents/skills"}
	}}`, 0o644)

	return f
}

const genericHasNoAgents = "generic has no agents directory"

// TestAIInitGenericWithTheDefaultResourcesInstallsSkillsAndSaysWhy: agents were only implied by
// the default, so the skills are installed, the command succeeds, and stderr says why no agent was.
func TestAIInitGenericWithTheDefaultResourcesInstallsSkillsAndSaysWhy(t *testing.T) {
	for _, resources := range [][]string{nil, {"--resources=skills,agents"}, {"--resources=both"}} {
		t.Run(strings.Join(append([]string{"resources"}, resources...), " "), func(t *testing.T) {
			f := genericTooling(t)

			args := append([]string{"--no-json", "ai-init", "--platforms=generic", "--types=hyva,magento"}, resources...)

			stdout, stderr, code := f.run(t, "", args...)
			if code != exitOK {
				t.Fatalf("%v = %d\nstdout: %s\nstderr: %s", args, code, stdout, stderr)
			}

			if !f.exists(".agents/skills/hyva-checkout") {
				t.Error("the skills were not installed in .agents/skills")
			}

			if !strings.Contains(stderr, genericHasNoAgents) {
				t.Errorf("stderr does not say generic has no agents directory:\n%s", stderr)
			}

			if strings.Contains(stdout, genericHasNoAgents) {
				t.Errorf("the note went to stdout:\n%s", stdout)
			}
		})
	}
}

// TestAIInitGenericWithOnlyAgentsIsAUsageError: nothing asked for can be installed, so the
// command is refused before anything is written.
func TestAIInitGenericWithOnlyAgentsIsAUsageError(t *testing.T) {
	for _, jsonOutput := range []string{"--no-json", "--json"} {
		t.Run(jsonOutput, func(t *testing.T) {
			f := genericTooling(t)

			stdout, stderr, code := f.run(t, "", jsonOutput, "ai-init", "--platforms=generic", "--resources=agents")
			if code != exitUsage {
				t.Fatalf("ai-init --platforms=generic --resources=agents = %d, want %d\nstdout: %s\nstderr: %s",
					code, exitUsage, stdout, stderr)
			}

			if !strings.Contains(stderr, genericHasNoAgents) {
				t.Errorf("stderr does not say generic has no agents directory:\n%s", stderr)
			}

			if f.exists(core.AIPropertiesFile) || f.exists(core.AIRegistrationFile) || f.exists(".agents") {
				t.Error("something was written for a command that was refused")
			}
		})
	}
}

// TestAIInitGenericBesideAPlatformWithAgentsInstallsTheirAgents: agents asked for alone are fine
// while one platform has a place for them; generic is the one said to have none.
func TestAIInitGenericBesideAPlatformWithAgentsInstallsTheirAgents(t *testing.T) {
	f := genericTooling(t)

	_, stderr, code := f.run(t, "", "--no-json", "ai-init", "--platforms=claude,generic", "--resources=agents")
	if code != exitOK {
		t.Fatalf("ai-init = %d\nstderr: %s", code, stderr)
	}

	if !f.exists(".claude/agents/reviewer.md") {
		t.Error("claude's agents were not installed")
	}

	if !strings.Contains(stderr, genericHasNoAgents) {
		t.Errorf("stderr does not say generic has no agents directory:\n%s", stderr)
	}
}

// TestAIInitSaysNothingWhenEveryPlatformHasEveryResource: the previous behaviour, where nothing is
// missing and nothing is said.
func TestAIInitSaysNothingWhenEveryPlatformHasEveryResource(t *testing.T) {
	f := genericTooling(t)

	_, stderr, code := f.run(t, "", "--no-json", "ai-init", "--platforms=claude", "--types=hyva")
	if code != exitOK {
		t.Fatalf("ai-init = %d\nstderr: %s", code, stderr)
	}

	if strings.Contains(stderr, "has no") {
		t.Errorf("stderr reports a directory missing:\n%s", stderr)
	}
}
