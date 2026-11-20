package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// ai-pull, in the words and streams of console/commands/ai-pull.sh.
//

const pulledConfiguration = `{"resources": ["skills", "agents"], "platforms": ["claude"], "types": ["hyva"], "custom_repositories": []}`

func TestAIPullInstallsTheToolsSkillsAndTheConfiguredTypes(t *testing.T) {
	f := aiTooling(t)
	f.configure(t, pulledConfiguration)

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-pull")
	if code != exitOK {
		t.Fatalf("ai-pull = %d\nstderr: %s", code, stderr)
	}

	// The tool's own skills, whatever the types; from the remote one, only the types asked for
	for name, want := range map[string]bool{
		".claude/skills/dockergento-environment": true, ".claude/skills/dockergento-database": true,
		".claude/skills/hyva-checkout": true, ".claude/skills/magento-orders": false,
		".claude/agents/reviewer.md": true,
	} {
		if f.exists(name) != want {
			t.Errorf("%s installed = %v, want %v", name, !want, want)
		}
	}

	registration := f.read(t, core.AIRegistrationFile)

	local := `    ".claude/skills/dockergento-database": {
      "checksum": "` + f.digest(".claude/skills/dockergento-database") + `",
      "installed": "2026-11-20 08:30:00",
      "origin": "dockergento",
      "url": "",
      "branch": "",
      "version": "2.0.0-test"
    }`
	remote := `    ".claude/skills/hyva-checkout": {
      "checksum": "` + f.digest(".claude/skills/hyva-checkout") + `",
      "installed": "2026-11-20 08:30:00",
      "origin": "remote-tools",
      "url": "https://github.com/acme/remote-tools",
      "branch": "main",
      "version": ""
    }`

	for _, entry := range []string{local, remote} {
		if !strings.Contains(registration, entry) {
			t.Errorf("registration has no entry\n%s\nin\n%s", entry, registration)
		}
	}

	if mode := f.mode(t, core.AIRegistrationFile); mode != 0o600 {
		t.Errorf("ai-registration.json mode = %o, want 600", mode)
	}

	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}

	want := lines(
		"Loading configuration...",
		"Configuration loaded:",
		"  Platforms: claude",
		"  Types: hyva",
		"  Resources: skills,agents",
		"",
		"Resolving repositories...",
		"Starting download and installation...",
		"Downloading from 2 repositories...",
		"Processing repository: dockergento",
		"Installing skills for claude...",
		"Installing dockergento-database...",
		"Installing dockergento-environment...",
		"Installed 2 skills",
		"Installing agents for claude...",
		"No agents found in repository",
		"Processing repository: remote-tools",
		"Downloading from https://github.com/acme/remote-tools/archive/refs/heads/main.tar.gz...",
		"Downloaded successfully",
		"Installing skills for claude...",
		"Installing hyva-checkout...",
		"Installed 1 skills",
		"Installing agents for claude...",
		"Installing reviewer...",
		"Installed 1 agents",
		"Processed 2 repositories (2 successful, 0 failed)",
		"",
		"AI tools update complete!",
		"",
		"Registration: config/docker/ai-registration.json",
		"",
		"Custom skills/agents were preserved",
		"Use --force to overwrite custom files",
	)
	if diff := cmp.Diff(want, stderr); diff != "" {
		t.Errorf("stderr (-want +got):\n%s", diff)
	}
}

// What the tool installed is replaced, edits and all; what somebody made is left alone.
func TestAIPullUpdatesWhatItInstalledAndKeepsWhatIsCustom(t *testing.T) {
	f := aiTooling(t)
	f.configure(t, pulledConfiguration)

	if _, stderr, code := f.run(t, "", "--no-json", "ai-pull"); code != exitOK {
		t.Fatalf("first ai-pull = %d\n%s", code, stderr)
	}

	plantFile(t, f.path(".claude/skills/dockergento-database/SKILL.md"), "edited\n", 0o644)
	plantFile(t, f.path(".claude/skills/dockergento-environment.bak/SKILL.md"), "mine\n", 0o644)
	plantFile(t, f.path(".claude/skills/hyva-mine/SKILL.md"), "mine\n", 0o644)
	// Same name as one the tool has, and never installed by it
	if err := os.Remove(f.path(".claude/agents/reviewer.md")); err != nil {
		t.Fatal(err)
	}
	plantFile(t, f.path(".claude/agents/reviewer.md"), "my reviewer\n", 0o644)
	registration := strings.Replace(f.read(t, core.AIRegistrationFile), `".claude/agents/reviewer.md"`, `"elsewhere.md"`, 1)
	plantFile(t, f.path(core.AIRegistrationFile), registration, 0o600)

	_, stderr, code := f.run(t, "", "--no-json", "ai-pull")
	if code != exitOK {
		t.Fatalf("ai-pull = %d\n%s", code, stderr)
	}

	if got := f.read(t, ".claude/skills/dockergento-database/SKILL.md"); got != "# database\n" {
		t.Errorf("the edited skill = %q, want it updated", got)
	}

	if got := f.read(t, ".claude/agents/reviewer.md"); got != "my reviewer\n" {
		t.Errorf("the custom agent = %q, want it kept", got)
	}

	for _, line := range []string{
		"Updating dockergento-database...\n",
		"Skipping reviewer (custom agent exists, use --force to overwrite)\n",
		"Skipped 1 custom agents\n",
	} {
		if !strings.Contains(stderr, line) {
			t.Errorf("stderr has no %q:\n%s", line, stderr)
		}
	}

	if !strings.Contains(f.read(t, core.AIRegistrationFile), `"checksum": "`+f.digest(".claude/skills/dockergento-database")+`"`) {
		t.Error("the updated skill's checksum was not recorded again")
	}
}

// --force is the tool's own flag: the router takes it, as bin/run did, so the command never sees
// it and a custom skill survives it.
func TestAIPullForceNeverReachesTheCommand(t *testing.T) {
	f := aiTooling(t)
	f.configure(t, pulledConfiguration)
	plantFile(t, f.path(".claude/skills/dockergento-database/SKILL.md"), "mine\n", 0o644)

	// The router exports HM_FORCE for the flag; registered here so it is put back afterwards
	t.Setenv("HM_FORCE", "")

	_, stderr, code := f.run(t, "", "--no-json", "ai-pull", "--force")
	if code != exitOK {
		t.Fatalf("ai-pull --force = %d\n%s", code, stderr)
	}

	if got := f.read(t, ".claude/skills/dockergento-database/SKILL.md"); got != "mine\n" {
		t.Errorf("the custom skill = %q, want it kept", got)
	}

	if !strings.Contains(stderr, "Custom skills/agents were preserved\n") {
		t.Errorf("stderr does not say custom files were preserved:\n%s", stderr)
	}
}

func TestAIPullWithoutConfigurationSaysToInit(t *testing.T) {
	f := aiTooling(t)

	_, stderr, code := f.run(t, "", "--no-json", "ai-pull")
	if code != exitError {
		t.Fatalf("ai-pull = %d, want 1", code)
	}

	if want := lines("Loading configuration...", "No configuration found. Run 'hm ai-init' first."); stderr != want {
		t.Errorf("stderr (-want +got):\n%s", cmp.Diff(want, stderr))
	}
}

func TestAIPullRefusesAnInvalidConfiguration(t *testing.T) {
	cases := map[string]string{
		"not JSON":          "{bad",
		"no resources":      `{"platforms": ["claude"]}`,
		"nothing to put in": `{"platforms": [], "resources": ["skills"], "types": []}`,
		"null":              "null",
		"a list":            `["claude"]`,
		"a string":          `"claude"`,
		"empty":             "\n",
	}

	wants := map[string]string{
		"not JSON":          "Failed to load configuration",
		"no resources":      "Invalid configuration in ai-properties.json",
		"nothing to put in": "Configuration incomplete: platforms and resources are required",
		"null":              "Invalid configuration in ai-properties.json",
		"a list":            "Invalid configuration in ai-properties.json",
		"a string":          "Invalid configuration in ai-properties.json",
		"empty":             "Invalid configuration in ai-properties.json",
	}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			f := aiTooling(t)
			f.configure(t, contents)

			_, stderr, code := f.run(t, "", "--no-json", "ai-pull")
			if code != exitError {
				t.Fatalf("ai-pull = %d, want 1", code)
			}

			if want := lines("Loading configuration...", wants[name]); stderr != want {
				t.Errorf("stderr (-want +got):\n%s", cmp.Diff(want, stderr))
			}

			if f.exists(".claude") {
				t.Error("something was installed")
			}
		})
	}
}

// A repository that cannot be downloaded is skipped and reported, and the command still ends well.
func TestAIPullGoesOnWhenADownloadFails(t *testing.T) {
	f := aiTooling(t)
	f.configure(t, pulledConfiguration)
	f.failDownload = true

	_, stderr, code := f.run(t, "", "--no-json", "ai-pull")
	if code != exitOK {
		t.Fatalf("ai-pull = %d, want 0", code)
	}

	if got := len(f.downloaded); got != 3 {
		t.Errorf("download attempts = %d, want 3", got)
	}

	for _, line := range []string{
		"Download attempt 3/3 failed\n",
		"Failed to download after 3 attempts\n",
		"Tarball download failed, trying git clone...\n",
		"Using git clone fallback for https://github.com/acme/remote-tools...\n",
		"Failed to clone after 3 attempts\n",
		"Failed to download remote-tools, skipping...\n",
		"Processed 2 repositories (1 successful, 1 failed)\n",
		"Some repositories failed to download or install\n",
		"Some operations failed, but continuing...\n",
	} {
		if !strings.Contains(stderr, line) {
			t.Errorf("stderr has no %q:\n%s", line, stderr)
		}
	}

	if !f.exists(".claude/skills/dockergento-database") {
		t.Error("the tool's own skills were not installed")
	}
}

func TestAIPullRefusesAnUnknownOption(t *testing.T) {
	f := aiTooling(t)

	stdout, stderr, code := f.run(t, "", "--json", "ai-pull", "--bogus")
	if code != exitUsage {
		t.Fatalf("ai-pull --bogus = %d, want 2", code)
	}

	if stderr != "Unknown option: --bogus\n" {
		t.Errorf("stderr = %q", stderr)
	}

	if stdout != aiPullUsage {
		t.Errorf("stdout (-want +got):\n%s", cmp.Diff(aiPullUsage, stdout))
	}
}

// A repository's name becomes a directory under the download area: a name that is a path is
// refused before anything is downloaded or written.
func TestAIPullRefusesARepositoryNameThatIsAPath(t *testing.T) {
	for _, name := range []string{"../escaped", "a/b", "/abs", "..", ".", "", `a\b`} {
		t.Run(name, func(t *testing.T) {
			f := aiTooling(t)
			f.configure(t, `{"resources": ["skills"], "platforms": ["claude"], "types": [],
				"custom_repositories": [{"name": "`+strings.ReplaceAll(name, `\`, `\\`)+`", "url": "https://github.com/acme/hostile", "branch": "main"}]}`)

			_, stderr, code := f.run(t, "", "--no-json", "ai-pull")
			if code != exitOK {
				t.Fatalf("ai-pull = %d\n%s", code, stderr)
			}

			for _, downloaded := range f.downloaded {
				if strings.Contains(downloaded, "hostile") {
					t.Errorf("downloaded %s", downloaded)
				}
			}

			if want := "Invalid repository name: " + name + ", skipping...\n"; !strings.Contains(stderr, want) {
				t.Errorf("stderr has no %q:\n%s", want, stderr)
			}
		})
	}
}

// An empty registration, or one missing a kind, holds nothing: it does not stop what is installed
// from being registered.
func TestAIPullRegistersIntoAnEmptyOrPartialRegistration(t *testing.T) {
	for _, registration := range []string{"", "\n", `{"skills": {}}`, `{"agents": {}}`} {
		t.Run(registration, func(t *testing.T) {
			f := aiTooling(t)
			f.configure(t, pulledConfiguration)
			plantFile(t, f.path(core.AIRegistrationFile), registration, 0o600)

			_, stderr, code := f.run(t, "", "--no-json", "ai-pull")
			if code != exitOK {
				t.Fatalf("ai-pull = %d\n%s", code, stderr)
			}

			if strings.Contains(stderr, "Failed to register") {
				t.Errorf("stderr:\n%s", stderr)
			}

			got := f.read(t, core.AIRegistrationFile)
			for _, path := range []string{`".claude/skills/hyva-checkout"`, `".claude/agents/reviewer.md"`} {
				if !strings.Contains(got, path) {
					t.Errorf("registration has no %s:\n%s", path, got)
				}
			}
		})
	}
}
