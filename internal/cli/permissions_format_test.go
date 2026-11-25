package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	hm "github.com/hiberus-magento/hiberus-dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// hm permissions --format: the same classification, in the configuration each agent reads.
//

// realPermissions is what the tool's own data files give, as the engine builds it.
func realPermissions(t *testing.T, strict bool) core.AgentPermissions {
	t.Helper()

	commands, err := fs.ReadFile(hm.Embedded(), "data/command_descriptions.json")
	if err != nil {
		t.Fatal(err)
	}

	exclusions, err := fs.ReadFile(hm.Embedded(), "data/ai-exclusions.json")
	if err != nil {
		t.Fatal(err)
	}

	permissions, err := core.ReadAgentPermissions(commands, exclusions, "hm", strict)
	if err != nil {
		t.Fatal(err)
	}

	return permissions
}

func permissionsGolden(t *testing.T, name string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join("testdata", "permissions", name))
	if err != nil {
		t.Fatal(err)
	}

	return string(contents)
}

// TestPermissionsForClaudeIsTodaysOutput: the goldens were captured from the binary before
// --format existed; the default and --format=claude both print exactly that.
func TestPermissionsForClaudeIsTodaysOutput(t *testing.T) {
	text, document := permissionsGolden(t, "claude.txt"), permissionsGolden(t, "claude.json")
	strict := permissionsGolden(t, "claude-strict.txt")

	for _, tc := range []struct {
		name   string
		args   []string
		strict bool
		want   string
	}{
		{"default text", []string{"--no-json", "permissions"}, false, text},
		{"claude text", []string{"--no-json", "permissions", "--format=claude"}, false, text},
		{"default json", []string{"--json", "permissions"}, false, document},
		{"claude json", []string{"--json", "permissions", "--format=claude"}, false, document},
		{"strict text", []string{"--no-json", "permissions", "--strict"}, true, strict},
		{"claude strict text", []string{"--no-json", "permissions", "--format=claude", "--strict"}, true, strict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := answering(t)
			fake.permissions = realPermissions(t, tc.strict)
			t.Setenv("NO_COLOR", "1")

			var stdout, stderr bytes.Buffer
			if code := Run(tc.args, &stdout, &stderr); code != exitOK {
				t.Fatalf("%v = %d\nstderr: %s", tc.args, code, stderr.String())
			}

			if diff := cmp.Diff(tc.want, stdout.String()); diff != "" {
				t.Errorf("stdout (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPermissionsRefusesAnUnknownFormat(t *testing.T) {
	for _, args := range [][]string{{"--format=cursor"}, {"--format="}, {"--format"}} {
		fake := answering(t)

		var stderr bytes.Buffer
		if code := permissions(args, io.Discard, &stderr, false); code != exitUsage {
			t.Errorf("permissions %v = %d, want exitUsage", args, code)
		}

		if len(fake.calls) != 0 {
			t.Errorf("permissions %v asked %v, want nothing", args, fake.calls)
		}

		if !strings.Contains(stderr.String(), "claude|codex|opencode") {
			t.Errorf("permissions %v does not say which formats there are:\n%s", args, stderr.String())
		}
	}
}

// TestPermissionsForCodexAreRules: Codex reads prefix_rule() calls from .codex/rules/*.rules, with
// allow, prompt or forbidden (https://learn.chatgpt.com/docs/agent-configuration/rules). It has no
// rule for paths, so those are listed for the reader instead of silently dropped.
func TestPermissionsForCodexAreRules(t *testing.T) {
	fake := answering(t)
	fake.permissions = agentPermissions()
	t.Setenv("NO_COLOR", "1")

	var stdout bytes.Buffer
	if code := permissions([]string{"--format=codex"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("permissions --format=codex = %d", code)
	}

	for _, want := range []string{
		`prefix_rule(pattern = ["hm", "describe"], decision = "allow",`,
		`prefix_rule(pattern = ["hm", "down"], decision = "prompt",`,
		"#   var/log/",
		".codex/rules/hm.rules",
		"1 command(s) allowed, 1 asking for confirmation.",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}

	if strings.Contains(stdout.String(), "Bash(") {
		t.Errorf("Claude's rules leaked into Codex's:\n%s", stdout.String())
	}
}

func TestPermissionsForCodexUnderJSONIsTheFileAndItsContents(t *testing.T) {
	fake := answering(t)
	fake.permissions = agentPermissions()

	var stdout bytes.Buffer
	if code := permissions([]string{"--format=codex"}, &stdout, io.Discard, true); code != exitOK {
		t.Fatalf("permissions --format=codex --json = %d", code)
	}

	var got struct {
		File  string `json:"file"`
		Rules string `json:"rules"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not a document: %v\n%s", err, stdout.String())
	}

	if got.File != ".codex/rules/hm.rules" || !strings.Contains(got.Rules, `decision = "prompt"`) {
		t.Errorf("document = %+v", got)
	}
}

// TestPermissionsForOpenCodeAreItsPermissions: OpenCode's permissions are an ordered list of
// action, resource and effect, the last match winning, and a shell pattern ending in " *" matching
// the command without arguments too (https://opencode.ai/docs/permissions).
func TestPermissionsForOpenCodeAreItsPermissions(t *testing.T) {
	fake := answering(t)
	fake.permissions = agentPermissions()

	var stdout bytes.Buffer
	if code := permissions([]string{"--format=opencode"}, &stdout, io.Discard, true); code != exitOK {
		t.Fatalf("permissions --format=opencode --json = %d", code)
	}

	want := `{
  "$schema": "https://opencode.ai/config.json",
  "permissions": [
    {
      "action": "shell",
      "resource": "hm describe *",
      "effect": "allow"
    },
    {
      "action": "shell",
      "resource": "hm down *",
      "effect": "ask"
    },
    {
      "action": "read",
      "resource": "var/log/*",
      "effect": "deny"
    }
  ]
}
`
	if diff := cmp.Diff(want, stdout.String()); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
}

func TestPermissionsForOpenCodeInTextSaysWhereItGoes(t *testing.T) {
	fake := answering(t)
	fake.permissions = agentPermissions()
	t.Setenv("NO_COLOR", "1")

	var stdout bytes.Buffer
	if code := permissions([]string{"--strict", "--format=opencode"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("permissions --strict --format=opencode = %d", code)
	}

	if len(fake.calls) != 1 || !fake.calls[0].All {
		t.Errorf("asked %v, want one strict AgentPermissions", fake.calls)
	}

	for _, want := range []string{"opencode.json", `"resource": "hm down *"`, "Strict: only commands with no side effects"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}
}
