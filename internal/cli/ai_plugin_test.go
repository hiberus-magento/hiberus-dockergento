package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// hm ai-init --plugin: a Claude Code plugin with the tool's skills, its MCP server and a hook that
// asks before a dangerous command, laid out as https://code.claude.com/docs/en/plugins-reference.md
// says: .claude-plugin/plugin.json, skills/, hooks/hooks.json and .mcp.json at the plugin root.
//

// pluginTooling is the AI fixture with the two data files permissions are built from.
func pluginTooling(t *testing.T) *aiFixture {
	t.Helper()

	f := aiTooling(t)
	plantFile(t, filepath.Join(f.tool, "data", "command_descriptions.json"), `{
		"_global": {"opts": []},
		"describe": {"safety": "safe"},
		"start": {"safety": "caution"},
		"down": {"safety": "dangerous"},
		"db": {"safety": "dangerous"}
	}`, 0o644)
	plantFile(t, filepath.Join(f.tool, "data", "ai-exclusions.json"),
		`{"exclusions": [{"path": "var/log", "reason": "logs"}]}`, 0o644)

	return f
}

func readJSONFile(t *testing.T, path string, into any) {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(contents, into); err != nil {
		t.Fatalf("%s is not JSON: %v\n%s", path, err, contents)
	}
}

type hooksFile struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			If      string `json:"if"`
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func TestAIInitPluginWritesTheBundle(t *testing.T) {
	f := pluginTooling(t)
	dir := filepath.Join(t.TempDir(), "plugin")

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-init", "--plugin="+dir)
	if code != exitOK {
		t.Fatalf("ai-init --plugin = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var manifest struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	readJSONFile(t, filepath.Join(dir, ".claude-plugin", "plugin.json"), &manifest)

	if manifest.Name != "hm" || manifest.Version != "2.0.0-test" || manifest.Description == "" {
		t.Errorf("plugin.json = %+v", manifest)
	}

	for _, skill := range []string{"dockergento-environment", "dockergento-database"} {
		if _, err := os.Stat(filepath.Join(dir, "skills", skill, "SKILL.md")); err != nil {
			t.Errorf("skill %s is not in the bundle: %v", skill, err)
		}
	}

	var mcp struct {
		Servers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	readJSONFile(t, filepath.Join(dir, ".mcp.json"), &mcp)

	server := mcp.Servers["hm"]
	if server.Command != "sh" || !strings.Contains(strings.Join(server.Args, " "), `cd "$CLAUDE_PROJECT_DIR" && exec hm mcp`) {
		t.Errorf(".mcp.json hm = %+v", server)
	}

	var hooks hooksFile
	readJSONFile(t, filepath.Join(dir, "hooks", "hooks.json"), &hooks)

	pre := hooks.Hooks["PreToolUse"]
	if len(pre) != 1 || pre[0].Matcher != "Bash" {
		t.Fatalf("PreToolUse = %+v, want one matcher for Bash", pre)
	}

	// One handler per dangerous command, with the very rule hm permissions asks about
	var conditions []string
	for _, handler := range pre[0].Hooks {
		conditions = append(conditions, handler.If)

		if handler.Type != "command" {
			t.Errorf("handler %+v is not a command", handler)
		}
	}

	if got, want := strings.Join(conditions, ","), "Bash(hm db:*),Bash(hm down:*)"; got != want {
		t.Errorf("hooks ask for %s, want %s", got, want)
	}

	// What the handler prints is the decision Claude Code reads: ask, with a reason
	output, err := exec.Command("sh", "-c", pre[0].Hooks[1].Command).Output()
	if err != nil {
		t.Fatalf("the hook does not run: %v", err)
	}

	var decision struct {
		Specific struct {
			Event    string `json:"hookEventName"`
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(output, &decision); err != nil {
		t.Fatalf("the hook prints %q, not JSON: %v", output, err)
	}

	if decision.Specific.Event != "PreToolUse" || decision.Specific.Decision != "ask" ||
		!strings.Contains(decision.Specific.Reason, "hm down") {
		t.Errorf("decision = %+v", decision.Specific)
	}

	if !strings.Contains(stdout, "claude --plugin-dir "+dir) {
		t.Errorf("stdout does not say how to load it:\n%s", stdout)
	}

	// Opt-in and nothing else: the project's AI configuration is not touched
	if f.exists(core.AIPropertiesFile) || f.exists(core.AIRegistrationFile) || f.exists(".claude/skills") {
		t.Error("ai-init --plugin configured or installed the project's AI tooling")
	}
}

func TestAIInitPluginGoesToTheToolsHomeByDefault(t *testing.T) {
	f := pluginTooling(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, stderr, code := f.run(t, "", "--no-json", "ai-init", "--plugin"); code != exitOK {
		t.Fatalf("ai-init --plugin = %d\nstderr: %s", code, stderr)
	}

	if _, err := os.Stat(filepath.Join(home, ".hm", "claude-plugin", ".claude-plugin", "plugin.json")); err != nil {
		t.Errorf("no bundle in ~/.hm/claude-plugin: %v", err)
	}
}

func TestAIInitPluginRefusesADirectoryThatIsNotItsOwn(t *testing.T) {
	f := pluginTooling(t)
	dir := t.TempDir()
	plantFile(t, filepath.Join(dir, "notes.txt"), "mine\n", 0o644)

	if _, _, code := f.run(t, "", "--no-json", "ai-init", "--plugin="+dir); code != exitError {
		t.Fatalf("ai-init --plugin over somebody's directory = %d, want exitError", code)
	}

	if _, err := os.Stat(filepath.Join(dir, ".claude-plugin")); err == nil {
		t.Error("the bundle was written over somebody's directory")
	}
}

// TestAIInitPluginReplacesItsOwnBundle: regenerating after an update drops a skill the tool no
// longer has, and leaves alone a file somebody added.
func TestAIInitPluginReplacesItsOwnBundle(t *testing.T) {
	f := pluginTooling(t)
	dir := filepath.Join(t.TempDir(), "plugin")

	if _, stderr, code := f.run(t, "", "--no-json", "ai-init", "--plugin="+dir); code != exitOK {
		t.Fatalf("first ai-init --plugin = %d\nstderr: %s", code, stderr)
	}

	plantFile(t, filepath.Join(dir, "skills", "retired", "SKILL.md"), "# retired\n", 0o644)
	plantFile(t, filepath.Join(dir, "notes.txt"), "mine\n", 0o644)

	if _, stderr, code := f.run(t, "", "--no-json", "ai-init", "--plugin="+dir); code != exitOK {
		t.Fatalf("second ai-init --plugin = %d\nstderr: %s", code, stderr)
	}

	if _, err := os.Stat(filepath.Join(dir, "skills", "retired")); err == nil {
		t.Error("a skill the tool no longer has is still in the bundle")
	}

	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Error("a file somebody added was removed")
	}

	if entries, err := os.ReadDir(filepath.Dir(dir)); err != nil || len(entries) != 1 {
		t.Errorf("beside the plugin there is %v (%v), want nothing left of building it", entries, err)
	}
}

// TestAIInitPluginLeavesThePreviousBundleWhenAWriteFails: the new bundle is built beside the old
// one and swapped in only once all of it is written, so a failure halfway leaves the plugin that
// was there whole, and nothing of the attempt behind.
func TestAIInitPluginLeavesThePreviousBundleWhenAWriteFails(t *testing.T) {
	f := pluginTooling(t)
	parent := t.TempDir()
	dir := filepath.Join(parent, "plugin")

	if _, stderr, code := f.run(t, "", "--no-json", "ai-init", "--plugin="+dir); code != exitOK {
		t.Fatalf("first ai-init --plugin = %d\nstderr: %s", code, stderr)
	}

	before := map[string]string{}

	for _, name := range []string{
		".claude-plugin/plugin.json", ".mcp.json", "hooks/hooks.json",
		"skills/dockergento-environment/SKILL.md", "skills/dockergento-database/SKILL.md",
	} {
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		before[name] = string(contents)
	}

	// A skill of the tool's that cannot be read: the copy fails after others were written
	unreadable := filepath.Join(f.tool, "skills", "dockergento-environment", "SKILL.md")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Chmod(unreadable, 0o644) }) //nolint:errcheck

	if _, err := os.ReadFile(unreadable); err == nil {
		t.Skip("the file can still be read (running as root?): no failure to provoke")
	}

	stdout, stderr, code := f.run(t, "", "--no-json", "ai-init", "--plugin="+dir)
	if code != exitError {
		t.Fatalf("ai-init --plugin with an unreadable skill = %d, want exitError\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	if !strings.Contains(stdout+stderr, "Failed to write the plugin") {
		t.Errorf("the failure is not reported:\nstdout: %s\nstderr: %s", stdout, stderr)
	}

	for name, want := range before {
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s of the previous plugin is gone: %v", name, err)

			continue
		}

		if string(contents) != want {
			t.Errorf("%s of the previous plugin changed:\n%s", name, contents)
		}
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name() != "plugin" {
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		t.Errorf("beside the plugin there is %v, want nothing left of the attempt", names)
	}
}

func TestAIInitPluginTakesNoOtherOption(t *testing.T) {
	f := pluginTooling(t)

	if _, _, code := f.run(t, "", "--no-json", "ai-init", "--plugin", "--platforms=claude"); code != exitUsage {
		t.Fatalf("ai-init --plugin --platforms = %d, want exitUsage", code)
	}

	if f.exists(core.AIPropertiesFile) {
		t.Error("the configuration was written")
	}
}
