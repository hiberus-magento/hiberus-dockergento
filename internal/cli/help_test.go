package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//
// The help, against what bin/run printed. The goldens in testdata/help were produced by the real
// shell implementation (testdata/help/capture.sh): a test here fails on any byte that differs.
//

// helpGoldens is where the goldens are, read before any test changes directory.
var helpGoldens, _ = filepath.Abs(filepath.Join("testdata", "help"))

// helpTool is this checkout: its data/command_descriptions.json is what the goldens describe.
var helpTool, _ = filepath.Abs(filepath.Join("..", ".."))

// helpIn runs the binary's entry point from dir, with or without colour, the way the goldens were
// captured: macOS, piped, nobody's own environment leaking in.
func helpIn(t *testing.T, dir string, colour bool, args ...string) (string, string, int) {
	t.Helper()

	pinHelp(t, helpTool, "mac")
	t.Chdir(dir)

	if colour {
		t.Setenv("FORCE_COLOR", "1")
	} else {
		t.Setenv("NO_COLOR", "1")
	}

	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)

	return stdout.String(), stderr.String(), code
}

func pinHelp(t *testing.T, tool, machine string) {
	t.Helper()

	for _, name := range []string{"NO_COLOR", "FORCE_COLOR", "CLICOLOR_FORCE", "HM_NO_COLOR", "HM_PROJECT_DIR",
		"HM_OUTPUT_FORMAT", "COMMAND_BIN_NAME"} {
		t.Setenv(name, "")
	}

	t.Setenv("TERM", "xterm-256color")

	originalRoot, originalMachine, originalTerminal := toolRoot, helpMachine, helpTerminal
	toolRoot = func() string { return tool }
	helpMachine = func() string { return machine }
	helpTerminal = func(io.Writer) bool { return false }

	t.Cleanup(func() { toolRoot, helpMachine, helpTerminal = originalRoot, originalMachine, originalTerminal })
}

// helpProject is the fixture project, copied somewhere that is not inside any repository: from a
// repository's subdirectory its commands would not be found, which is the shell's rule.
func helpProject(t *testing.T) string {
	t.Helper()

	project := t.TempDir()
	commands := filepath.Join(project, "config", "hm", "commands")

	if err := os.MkdirAll(commands, 0o755); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(helpGoldens, "project", "config", "hm", "commands")

	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		contents, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(commands, entry.Name()), contents, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	return project
}

func golden(t *testing.T, name string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(helpGoldens, name))
	if err != nil {
		t.Fatal(err)
	}

	return string(contents)
}

func assertGolden(t *testing.T, name, stdout, stderr string, code int) {
	t.Helper()

	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q, want 0 and nothing", code, stderr)
	}

	if want := golden(t, name); stdout != want {
		t.Fatalf("%s differs from bin/run's\n got: %q\nwant: %q", name, stdout, want)
	}
}

func colourSuffix(colour bool) string {
	if colour {
		return ".colour.txt"
	}

	return ".txt"
}

func TestTheListOfCommandsIsTheShells(t *testing.T) {
	for _, colour := range []bool{false, true} {
		t.Run(colourSuffix(colour), func(t *testing.T) {
			stdout, stderr, code := helpIn(t, t.TempDir(), colour, "--help")
			assertGolden(t, "full"+colourSuffix(colour), stdout, stderr, code)
		})
	}
}

func TestNothingAtAllAndDashHAreTheListToo(t *testing.T) {
	for _, args := range [][]string{{}, {"-h"}, {"-h", "describe"}, {"--json", "--help"}, {"--help", "--no-color"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, code := helpIn(t, t.TempDir(), false, args...)
			assertGolden(t, "full.txt", stdout, stderr, code)
		})
	}
}

func TestEveryCommandsHelpIsTheShells(t *testing.T) {
	pages, err := filepath.Glob(filepath.Join(helpGoldens, "commands", "*.txt"))
	if err != nil || len(pages) < 2*65 {
		t.Fatalf("%d goldens (%v), want two for each of the 65 commands", len(pages), err)
	}

	for _, page := range pages {
		file := filepath.Base(page)
		colour := strings.HasSuffix(file, ".colour.txt")
		name := strings.TrimSuffix(strings.TrimSuffix(file, ".txt"), ".colour")

		t.Run(file, func(t *testing.T) {
			stdout, stderr, code := helpIn(t, t.TempDir(), colour, name, "--help")
			assertGolden(t, filepath.Join("commands", file), stdout, stderr, code)
		})
	}
}

func TestDashHAfterTheCommandAsksForItsHelpToo(t *testing.T) {
	stdout, stderr, code := helpIn(t, t.TempDir(), false, "--json", "db", "-h")
	assertGolden(t, filepath.Join("commands", "db.txt"), stdout, stderr, code)
}

func TestTheProjectsOwnCommandsAreListed(t *testing.T) {
	for _, colour := range []bool{false, true} {
		t.Run(colourSuffix(colour), func(t *testing.T) {
			stdout, stderr, code := helpIn(t, helpProject(t), colour, "--help")
			assertGolden(t, "project"+colourSuffix(colour), stdout, stderr, code)
		})
	}
}

func TestAProjectCommandsHelpComesFromItsOwnDescriptions(t *testing.T) {
	for _, page := range []string{"project-deploy", "project-hello"} {
		for _, colour := range []bool{false, true} {
			t.Run(page+colourSuffix(colour), func(t *testing.T) {
				name := strings.TrimPrefix(page, "project-")
				stdout, stderr, code := helpIn(t, helpProject(t), colour, name, "--help")
				assertGolden(t, page+colourSuffix(colour), stdout, stderr, code)
			})
		}
	}
}

func TestACommandDescribedNowhereIsOneEmptyLine(t *testing.T) {
	stdout, stderr, code := helpIn(t, t.TempDir(), false, "nonexistent", "--help")
	assertGolden(t, "unknown.txt", stdout, stderr, code)
}

func TestTheMacOnlyCommandsAreNotListedElsewhere(t *testing.T) {
	pinHelp(t, helpTool, "linux")
	t.Chdir(t.TempDir())
	t.Setenv("NO_COLOR", "1")

	var stdout, stderr bytes.Buffer
	Run([]string{"--help"}, &stdout, &stderr)

	if strings.Contains(stdout.String(), "copy-to-container") {
		t.Fatal("copy-to-container listed on linux, want it only on mac")
	}

	if !strings.Contains(stdout.String(), "transfer-media") {
		t.Fatal("the rest of the Files group is gone too, want only the mac-only commands dropped")
	}
}

// A tool of the test's own, for the rules the real data file does not exercise.
func toolDescribing(t *testing.T, descriptions string) string {
	t.Helper()

	tool := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tool, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tool, "data", "command_descriptions.json"), []byte(descriptions), 0o644); err != nil {
		t.Fatal(err)
	}

	return tool
}

func TestACommandWithNoGroupIsListedUnderOther(t *testing.T) {
	pinHelp(t, toolDescribing(t, `{
		"_groups": [{"id": "environment", "title": "Environment"}, {"id": "other", "title": "Other"}],
		"start": {"description": "Start", "group": "environment"},
		"stray": {"description": "Nobody grouped this"}
	}`), "mac")
	t.Chdir(t.TempDir())
	t.Setenv("NO_COLOR", "1")

	var stdout, stderr bytes.Buffer
	Run([]string{"--help"}, &stdout, &stderr)

	want := "Usage: hm <command> [options]\n\nEnvironment\n  start                 Start\n\nOther\n" +
		"  stray                 Nobody grouped this\nRun hm <command> --help for details on a single command.\n\n"
	if stdout.String() != want {
		t.Fatalf("got %q\nwant %q", stdout.String(), want)
	}
}

func TestTheLogoIsForATerminalOnly(t *testing.T) {
	for _, tc := range []struct {
		locale, want, not string
	}{
		{"en_US.UTF-8", "█  █ █▀▄▀█   Hiberus Dockergento\n", "HHHHH"},
		{"C", "HHHHH  M M M   Docker environments for Magento 2\n", "█"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			pinHelp(t, helpTool, "mac")
			helpTerminal = func(io.Writer) bool { return true }
			t.Chdir(t.TempDir())
			t.Setenv("NO_COLOR", "1")
			t.Setenv("LC_ALL", tc.locale)

			var stdout, stderr bytes.Buffer
			Run([]string{"--help"}, &stdout, &stderr)

			if !strings.HasPrefix(stdout.String(), tc.want[:strings.Index(tc.want, "\n")]) &&
				!strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("no logo %q in %q", tc.want, stdout.String()[:200])
			}

			if strings.Contains(stdout.String(), tc.not) {
				t.Fatalf("%q drawn under %s", tc.not, tc.locale)
			}

			if !strings.Contains(stdout.String(), "Hiberus Dockergento\n") ||
				!strings.Contains(stdout.String(), "\n\nUsage: ") {
				t.Fatalf("logo not followed by an empty line and the usage: %q", stdout.String()[:200])
			}
		})
	}
}

func TestThePrintfOfTheShellIsTheShells(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`a\tb\n`, "a\tb\n"},
		{`100%% sure %s.%5d|%-3s|`, "100% sure .    0|   |"},
		{`\101\x41é`, "AAé"},
	} {
		if got := bashFormat(tc.in); got != tc.want {
			t.Fatalf("bashFormat(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	if got, stopped := expandB(`one\ntwo\cthree`); got != "one\ntwo" || !stopped {
		t.Fatalf("expandB = %q %v, want the output to stop at \\c", got, stopped)
	}

	if got := bashB(`\0101\e[0m\q`); got != "A\x1b[0m\\q" {
		t.Fatalf("bashB = %q", got)
	}
}
