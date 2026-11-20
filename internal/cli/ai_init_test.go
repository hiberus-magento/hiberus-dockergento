package cli

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// ai-init, in the words and streams of console/commands/ai-init.sh.
//

func TestAIInitWithOptionsConfiguresAndInstalls(t *testing.T) {
	f := aiTooling(t)

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-init",
		"--platforms=claude", "--types=hyva,magento", "--resources=skills,agents")
	if code != exitOK {
		t.Fatalf("ai-init = %d\nstderr: %s", code, stderr)
	}

	wantConfig := `{
  "resources": [
    "skills",
    "agents"
  ],
  "platforms": [
    "claude"
  ],
  "types": [
    "hyva",
    "magento"
  ],
  "custom_repositories": [],
  "configured_at": "2026-11-20 08:30:00"
}
`
	if diff := cmp.Diff(wantConfig, f.read(t, core.AIPropertiesFile)); diff != "" {
		t.Errorf("ai-properties.json (-want +got):\n%s", diff)
	}

	// Written through mktemp, so readable by its owner only
	if mode := f.mode(t, core.AIPropertiesFile); mode != 0o600 {
		t.Errorf("ai-properties.json mode = %o, want 600", mode)
	}

	// Only the types asked for, with their files' modes
	for name, want := range map[string]bool{
		".claude/skills/hyva-checkout": true, ".claude/skills/magento-orders": true,
		".claude/skills/other-thing": false, ".claude/agents/reviewer.md": true,
	} {
		if f.exists(name) != want {
			t.Errorf("%s installed = %v, want %v", name, !want, want)
		}
	}

	if mode := f.mode(t, ".claude/skills/magento-orders/scripts/run.sh"); mode != 0o755 {
		t.Errorf("run.sh mode = %o, want 755", mode)
	}

	entry := func(path string) string {
		return `    "` + path + `": {
      "checksum": "` + f.digest(path) + `",
      "installed": "2026-11-20 08:30:00",
      "origin": "",
      "url": "",
      "branch": "",
      "version": ""
    }`
	}
	wantRegistration := "{\n  \"skills\": {\n" + entry(".claude/skills/hyva-checkout") + ",\n" +
		entry(".claude/skills/magento-orders") + "\n  },\n  \"agents\": {\n" +
		entry(".claude/agents/reviewer.md") + "\n  }\n}\n"

	if diff := cmp.Diff(wantRegistration, f.read(t, core.AIRegistrationFile)); diff != "" {
		t.Errorf("ai-registration.json (-want +got):\n%s", diff)
	}

	if mode := f.mode(t, core.AIRegistrationFile); mode != 0o600 {
		t.Errorf("ai-registration.json mode = %o, want 600", mode)
	}

	wantOut := "Saving configuration...\n" +
		"Configuration saved to config/docker/ai-properties.json\n" +
		"Creating platform directories...\n" +
		"Resolving repositories...\n" +
		"Starting download and installation...\n" +
		"Downloading from 2 repositories...\n" +
		"Processing repository: dockergento\n" +
		// print_warning, which adds no newline
		"Failed to download dockergento, skipping..." +
		"Processing repository: remote-tools\n" +
		"Installing skills for claude...\n" +
		"Installing agents for claude...\n" +
		"Download and installation complete\n" +
		"\n" +
		"AI tools initialization complete!\n" +
		"\n" +
		"Configuration: config/docker/ai-properties.json\n" +
		"Registration: config/docker/ai-registration.json\n" +
		"\n" +
		"You can now use AI tools with your configured platforms.\n" +
		"Run 'hm ai-pull' to update tools in the future.\n"
	if diff := cmp.Diff(wantOut, stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
}

// ai-init's installer predates the tool's own skills: it tries to download the local repository,
// which has no URL, and skips it. ai-pull is what installs those.
func TestAIInitSkipsTheToolsOwnRepository(t *testing.T) {
	f := aiTooling(t)

	_, stderr, code := f.run(t, "", "--no-json", "ai-init", "--platforms=claude", "--resources=skills")
	if code != exitOK {
		t.Fatalf("ai-init = %d", code)
	}

	if f.exists(".claude/skills/dockergento-environment") {
		t.Error("the tool's own skills were installed by ai-init")
	}

	want := lines(
		"Repository URL must use HTTPS protocol: null",
		"Tarball download failed, trying git clone...",
		"Repository URL must use HTTPS protocol: null",
		"Downloading from https://github.com/acme/remote-tools/archive/refs/heads/main.tar.gz...",
		"Downloaded successfully",
		"Installing hyva-checkout...",
		"Installing magento-orders...",
		"Installing other-thing...",
		"Installed 3 skills",
	)
	if diff := cmp.Diff(want, stderr); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}
}

func TestAIInitAsksWhatIsNotGiven(t *testing.T) {
	f := aiTooling(t)
	t.Setenv("HM_NON_INTERACTIVE", "")

	answers := "1\nclaude, 2\nhyva\ny\nhttps://github.com/acme/tools.git\n\n\n"

	_, stderr, code := f.run(t, answers, "--no-json", "ai-init")
	if code != exitOK {
		t.Fatalf("ai-init = %d\nstderr: %s", code, stderr)
	}

	want := `{
  "resources": [
    "skills"
  ],
  "platforms": [
    "claude",
    "cursor"
  ],
  "types": [
    "hyva"
  ],
  "custom_repositories": [
    {
      "name": "tools",
      "url": "https://github.com/acme/tools.git",
      "branch": "main",
      "types": []
    }
  ],
  "configured_at": "2026-11-20 08:30:00"
}
`
	if diff := cmp.Diff(want, f.read(t, core.AIPropertiesFile)); diff != "" {
		t.Errorf("ai-properties.json (-want +got):\n%s", diff)
	}

	wantMenu := "\nWhich AI platforms do you use?\n" +
		"  [ ] 1) Claude Code - Claude Code (CLI, Desktop, Web)\n" +
		"  [ ] 2) Cursor AI Editor - Cursor AI Editor\n\n> "
	if !strings.Contains(stderr, wantMenu) {
		t.Errorf("stderr has no platform menu %q:\n%s", wantMenu, stderr)
	}

	if !f.exists(".cursor/skills/hyva-checkout") {
		t.Error("the answers were not what was installed")
	}
}

// Reconfiguring starts from what is there: an empty answer keeps it, and it is shown ticked.
func TestAIInitWizardKeepsTheExistingConfiguration(t *testing.T) {
	f := aiTooling(t)
	t.Setenv("HM_NON_INTERACTIVE", "")
	f.configure(t, `{"resources": ["agents"], "platforms": ["cursor"], "types": [], "custom_repositories": []}`)

	_, stderr, code := f.run(t, "\n\n\nn\n", "--no-json", "ai-init")
	if code != exitOK {
		t.Fatalf("ai-init = %d\nstderr: %s", code, stderr)
	}

	if !strings.Contains(stderr, "  [X] 2) Cursor AI Editor - Cursor AI Editor\n") {
		t.Errorf("the configured platform is not ticked:\n%s", stderr)
	}

	config := f.read(t, core.AIPropertiesFile)
	if !strings.Contains(config, "\"platforms\": [\n    \"cursor\"\n  ]") || !strings.Contains(config, "\"resources\": [\n    \"agents\"\n  ]") {
		t.Errorf("the configuration was not kept:\n%s", config)
	}
}

func TestAIInitWithNobodyToAskRefusesEachQuestion(t *testing.T) {
	f := aiTooling(t)

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-init")
	if code != exitError {
		t.Fatalf("ai-init = %d, want 1", code)
	}

	refusal := "> \nNon-interactive mode cannot run the AI tools wizard\n\n  hm ai-init --platforms=... --types=... --resources=...\n\n"
	if got := strings.Count(stderr, refusal); got != 3 {
		t.Errorf("refusals = %d, want one per menu\n%s", got, stderr)
	}

	if want := "Configuration incomplete: platforms and resources are required"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if f.exists(core.AIPropertiesFile) {
		t.Error("a configuration was written")
	}
}

func TestAIInitUnderJSONRefusesWithEnvelopes(t *testing.T) {
	f := aiTooling(t)

	stdout, stderr, code := f.run(t, "", "--json", "ai-init")
	if code != exitError {
		t.Fatalf("ai-init = %d, want 1", code)
	}

	envelope := `{
  "schema_version": 1,
  "command": "ai-init",
  "ok": false,
  "error": {
    "code": 2,
    "type": "input_required",
    "message": "Non-interactive mode cannot run the AI tools wizard",
    "hint": "hm ai-init --platforms=... --types=... --resources=..."
  }
}
`
	if got := strings.Count(stderr, envelope); got != 3 {
		t.Errorf("envelopes = %d, want 3\n%s", got, stderr)
	}

	if !strings.HasSuffix(stderr, "Configuration incomplete: platforms and resources are required") {
		t.Errorf("stderr does not end with the complaint:\n%s", stderr)
	}

	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
}

func TestAIInitRefusesAnUnknownOption(t *testing.T) {
	f := aiTooling(t)

	stdout, _, code := f.run(t, "", "--no-json", "ai-init", "--platforms")
	if code != exitUsage {
		t.Fatalf("ai-init --platforms = %d, want 2", code)
	}

	if want := "Unknown option: --platforms" + aiInitUsage; stdout != want {
		t.Errorf("stdout (-want +got):\n%s", cmp.Diff(want, stdout))
	}

	if len(f.fake.calls) != 0 {
		t.Errorf("calls = %v, want none", f.fake.calls)
	}
}

func TestAIInitRefusesAConfigurationThatIsNotJSON(t *testing.T) {
	f := aiTooling(t)
	f.configure(t, "{bad")

	stdout, _, code := f.run(t, "", "--no-json", "ai-init", "--platforms=claude")
	if code != exitError {
		t.Fatalf("ai-init = %d, want 1", code)
	}

	if want := "Failed to load existing configuration"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if got := f.read(t, core.AIPropertiesFile); got != "{bad" {
		t.Errorf("the configuration was rewritten: %q", got)
	}
}
