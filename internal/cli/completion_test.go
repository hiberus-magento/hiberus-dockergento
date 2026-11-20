package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// declaredCommands is every command data/command_descriptions.json declares, sorted.
func declaredCommands(t *testing.T) []string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(helpTool, "data", "command_descriptions.json"))
	if err != nil {
		t.Fatal(err)
	}

	var descriptions map[string]any
	if err := json.Unmarshal(contents, &descriptions); err != nil {
		t.Fatal(err)
	}

	var commands []string
	for name := range descriptions {
		if !strings.HasPrefix(name, "_") {
			commands = append(commands, name)
		}
	}

	sort.Strings(commands)

	return commands
}

func runCompletion(t *testing.T, machine string, args ...string) (string, string, int) {
	t.Helper()

	pinHelp(t, helpTool, machine)

	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"completion"}, args...), &stdout, &stderr)

	return stdout.String(), stderr.String(), code
}

// The bash script is what generate_completion.sh wrote into console/hm-completion.bash on macOS:
// every declared command, one per line of the word list, in jq's order.
func TestTheBashCompletionListsEveryCommand(t *testing.T) {
	stdout, stderr, code := runCompletion(t, "mac", "bash")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	want := "#!/usr/bin/env bash\n\ncomplete -W \"" + strings.Join(declaredCommands(t), "\n") + "\n\" hm\n"
	if stdout != want {
		t.Fatalf("got\n%s\nwant\n%s", stdout, want)
	}

	if !strings.Contains(stdout, "\ncompletion\n") {
		t.Error("the completion does not offer itself")
	}
}

func TestTheZshCompletionListsEveryCommand(t *testing.T) {
	stdout, stderr, code := runCompletion(t, "mac", "zsh")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	if !strings.HasPrefix(stdout, "#compdef hm\n") || !strings.Contains(stdout, "compdef _hm hm") {
		t.Errorf("not a zsh completion for hm:\n%s", stdout)
	}

	if !strings.Contains(stdout, "compadd -- "+strings.Join(declaredCommands(t), " ")+"\n") {
		t.Errorf("the zsh completion does not offer every command:\n%s", stdout)
	}
}

// On Linux the two commands of the macOS vendor dance are left out, as the generator left them.
func TestOnLinuxTheMacOnlyCommandsAreNotOffered(t *testing.T) {
	stdout, _, _ := runCompletion(t, "linux", "bash")

	for _, command := range []string{"copy-to-container", "copy-from-container"} {
		if strings.Contains(stdout, command) {
			t.Errorf("%s offered on linux", command)
		}
	}

	if !strings.Contains(stdout, "\nstart\n") {
		t.Errorf("the other commands are missing:\n%s", stdout)
	}
}

// The list is the declarations the binary reads, so a test handing its own gets its own.
func TestTheCompletionIsBuiltFromTheToolsAssets(t *testing.T) {
	pinHelp(t, helpTool, "mac")
	toolAssets = func() fs.FS {
		return fstest.MapFS{"data/command_descriptions.json": {Data: []byte(`{"_groups": [], "zeta": {}, "alpha": {}}`)}}
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"completion", "bash"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	if want := "#!/usr/bin/env bash\n\ncomplete -W \"alpha\nzeta\n\" hm\n"; stdout.String() != want {
		t.Errorf("got %q, want %q", stdout.String(), want)
	}
}

func TestTheCompletionNeedsAShellItKnows(t *testing.T) {
	for _, args := range [][]string{{}, {"fish"}, {"bash", "zsh"}} {
		_, stderr, code := runCompletion(t, "mac", args...)
		if code != exitUsage {
			t.Errorf("completion %v = exit %d, want %d", args, code, exitUsage)
		}

		if !strings.Contains(stderr, "bash or zsh") {
			t.Errorf("completion %v said %q", args, stderr)
		}
	}
}
