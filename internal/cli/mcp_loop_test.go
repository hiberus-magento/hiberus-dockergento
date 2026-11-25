package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

//
// The Magento loop tools: run_tests, xdebug, after_change and magento_exceptions. Each runs
// commands of this binary, so each is proved against a recorder standing in for it, which answers
// with an output and an exit status.
//

// looping is a server whose two doors, with and without the exit status, both answer from answer
// and record into one log, in the order asked.
func looping(write bool, answer func(args []string) (string, int)) (*[]ranCall, mcpServer) {
	calls := &[]ranCall{}

	record := func(dir string, args []string, combined bool) (string, int) {
		*calls = append(*calls, ranCall{args: append([]string(nil), args...), combined: combined, dir: dir})

		return answer(args)
	}

	return calls, mcpServer{
		write: write,
		capture: func(dir string, args []string, combined bool) []byte {
			output, _ := record(dir, args, combined)

			return []byte(output)
		},
		captureStatus: func(dir string, args []string, combined bool) ([]byte, int) {
			output, status := record(dir, args, combined)

			return []byte(output), status
		},
	}
}

// called is the arguments of each run, in order.
func called(calls []ranCall) [][]string {
	var all [][]string
	for _, one := range calls {
		all = append(all, one.args)
	}

	return all
}

// structured is the structuredContent of the one tools/call result in out, and its error flag.
func structured(t *testing.T, out string) (map[string]any, bool) {
	t.Helper()

	result := decoded(t, out).Result
	content, _ := result["structuredContent"].(map[string]any)
	isError, _ := result["isError"].(bool)

	return content, isError
}

const junitPassing = `<testsuites><testsuite name="Unit"><testcase name="testA" class="A" time="0.5"/><testcase name="testB" class="A" time="0.5"/></testsuite></testsuites>`

const junitFailing = `<testsuites><testsuite name="Unit"><testcase name="testA" class="A" time="0.5"/>` +
	`<testcase name="testB" class="A" time="0.5"><failure type="F">A::testB` + "\n" + `Failed asserting that false is true.</failure></testcase></testsuite></testsuites>`

func TestRunTestsReadsPHPUnitsReport(t *testing.T) {
	t.Run("the counts and the failures come from the report", func(t *testing.T) {
		quiet(t)

		var junit string

		calls, server := looping(false, func(args []string) (string, int) {
			switch {
			case len(args) > 1 && args[1] == "test-unit":
				junit = args[3]

				return "PHPUnit 10\n..F\nFAILURES!\n", 1
			case len(args) > 2 && args[2] == "cat":
				return junitFailing, 0
			}

			return "", 0
		})

		content, isError := structured(t, serving(server, toolCall("run_tests", `{"path":"app/code/Acme/Shop/Test/Unit"}`)))

		if !strings.HasPrefix(junit, "/tmp/hm-mcp-junit-") || !strings.HasSuffix(junit, ".xml") {
			t.Fatalf("report path = %q, want one in the container's /tmp", junit)
		}

		want := [][]string{
			{"--no-json", "test-unit", "--log-junit", junit, "app/code/Acme/Shop/Test/Unit"},
			{"--no-json", "exec", "cat", junit},
			{"--no-json", "exec", "rm", "-f", junit},
		}
		if diff := cmp.Diff(want, called(*calls)); diff != "" {
			t.Fatalf("runs (-want +got):\n%s", diff)
		}

		if isError || content["passed"] != false || content["source"] != "junit" || content["exit_code"] != float64(1) ||
			content["tests"] != float64(2) || content["failures"] != float64(1) || content["suite"] != "unit" {
			t.Fatalf("content = %v, want a failed run read from the report", content)
		}

		failed, _ := content["failed"].([]any)
		if len(failed) != 1 || failed[0].(map[string]any)["message"] != "Failed asserting that false is true." {
			t.Fatalf("failed = %v, want the failure and its message", content["failed"])
		}

		if _, there := content["tail"]; there {
			t.Errorf("tail = %v, want none: the report says why", content["tail"])
		}
	})

	t.Run("a passing run passes", func(t *testing.T) {
		quiet(t)

		_, server := looping(false, func(args []string) (string, int) {
			if len(args) > 2 && args[2] == "cat" {
				return junitPassing, 0
			}

			return "OK (2 tests)\n", 0
		})

		content, _ := structured(t, serving(server, toolCall("run_tests", `{"suite":"integration"}`)))

		if content["passed"] != true || content["tests"] != float64(2) || content["suite"] != "integration" {
			t.Fatalf("content = %v, want a pass", content)
		}
	})

	t.Run("the integration suite runs test-integration", func(t *testing.T) {
		quiet(t)
		calls, server := looping(false, func([]string) (string, int) { return "", 0 })

		serving(server, toolCall("run_tests", `{"suite":"integration"}`))

		if got := (*calls)[0].args[1]; got != "test-integration" {
			t.Fatalf("ran %v, want test-integration", (*calls)[0].args)
		}
	})
}

func TestRunTestsWithoutAReportSaysTheExitCodeAndTheTail(t *testing.T) {
	quiet(t)

	var output strings.Builder
	for line := range 60 {
		output.WriteString("line " + string(rune('A'+line%26)) + "\n")
	}

	output.WriteString("PHP Fatal error: bootstrap failed\n")

	_, server := looping(false, func(args []string) (string, int) {
		if len(args) > 1 && args[1] == "test-unit" {
			return output.String(), 255
		}

		// No report was written: cat fails and prints nothing on stdout
		return "", 1
	})

	content, isError := structured(t, serving(server, toolCall("run_tests", `{}`)))

	if isError || content["passed"] != false || content["source"] != "exit code" || content["exit_code"] != float64(255) {
		t.Fatalf("content = %v, want the exit code as the answer", content)
	}

	tail, _ := content["tail"].(string)
	if lines := strings.Split(tail, "\n"); len(lines) != 40 || lines[39] != "PHP Fatal error: bootstrap failed" {
		t.Fatalf("tail = %q, want the last forty lines", tail)
	}

	if _, there := content["tests"]; there {
		t.Errorf("tests = %v, want no count invented without a report", content["tests"])
	}
}

// TestRunTestsRefusesAPathThatIsNotOne: the path goes into a shell command line inside the
// container, so anything but a plain relative path is refused before anything runs.
func TestRunTestsRefusesAPathThatIsNotOne(t *testing.T) {
	for _, arguments := range []string{
		`{"path":"x; rm -rf /"}`,
		`{"path":"$(id)"}`,
		`{"path":"../../etc/passwd"}`,
		`{"path":"--filter=x"}`,
		`{"path":"/etc/passwd"}`,
		`{"path":"a b"}`,
		`{"suite":"e2e"}`,
	} {
		t.Run(arguments, func(t *testing.T) {
			quiet(t)

			text, isError := answer(t, serving(mcpServer{capture: refusing(t), captureStatus: refusingStatus(t)},
				toolCall("run_tests", arguments)))

			if !isError || text == "" {
				t.Fatalf("= %q, %v; want a refusal that says why", text, isError)
			}
		})
	}
}

// TestTheStatusDoorSaysTheExitStatus: run_tests and after_change decide by it, so it is the
// command's own, and a binary that cannot be started is -1 and says so.
func TestTheStatusDoorSaysTheExitStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hm")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho out\necho err >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if output, status := mcpCaptureStatus(path, "", nil, true); string(output) != "out\nerr\n" || status != 3 {
		t.Fatalf("= %q, %d; want both streams and 3", output, status)
	}

	output, status := mcpCaptureStatus(filepath.Join(t.TempDir(), "missing"), "", nil, false)
	if status != -1 || !strings.Contains(string(output), "could not be run") {
		t.Fatalf("= %q, %d; want it said, and -1", output, status)
	}
}

func refusingStatus(t *testing.T) func(string, []string, bool) ([]byte, int) {
	return func(_ string, args []string, _ bool) ([]byte, int) {
		t.Fatalf("ran %v, want nothing run", args)

		return nil, 0
	}
}

func TestXdebugSaysWhetherItIsOn(t *testing.T) {
	quiet(t)

	calls, server := looping(false, func([]string) (string, int) {
		return `{"ok":true,"data":{"project":{"name":"tienda"},"tooling":{"xdebug":"on"}}}`, 0
	})

	content, isError := structured(t, serving(server, toolCall("xdebug", `{}`)))

	if isError || !cmp.Equal(map[string]any{"xdebug": "on"}, content) {
		t.Fatalf("content = %v, %v; want xdebug on", content, isError)
	}

	if diff := cmp.Diff([][]string{{"describe", "--json"}}, called(*calls)); diff != "" {
		t.Fatalf("runs (-want +got):\n%s", diff)
	}
}

func TestXdebugTurnsItOnOrOffOnlyWithWrite(t *testing.T) {
	t.Run("without --write, turning it is refused and nothing runs", func(t *testing.T) {
		quiet(t)

		text, isError := answer(t, serving(mcpServer{capture: refusing(t), captureStatus: refusingStatus(t)},
			toolCall("xdebug", `{"state":"on"}`)))

		if !isError || !strings.Contains(text, "--write") {
			t.Fatalf("= %q, %v; want a refusal naming --write", text, isError)
		}
	})

	for state, command := range map[string]string{"on": "debug-on", "off": "debug-off"} {
		t.Run("with --write, "+state+" is "+command, func(t *testing.T) {
			quiet(t)
			calls, server := looping(true, func([]string) (string, int) { return "  xdebug enabled\n", 0 })

			text, isError := answer(t, serving(server, toolCall("xdebug", `{"state":"`+state+`"}`)))

			if isError || text != "  xdebug enabled" {
				t.Fatalf("= %q, %v; want what the command printed", text, isError)
			}

			want := []ranCall{{args: []string{"--no-json", command}, combined: true}}
			if diff := cmp.Diff(want, *calls, cmp.AllowUnexported(ranCall{})); diff != "" {
				t.Fatalf("runs (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("a state that is neither is refused", func(t *testing.T) {
		quiet(t)

		_, isError := answer(t, serving(mcpServer{write: true, capture: refusing(t), captureStatus: refusingStatus(t)},
			toolCall("xdebug", `{"state":"maybe"}`)))

		if !isError {
			t.Fatal("isError = false, want the state refused")
		}
	})
}

func TestAfterChangeSaysWhatIsNeededAndRunsNothing(t *testing.T) {
	quiet(t)

	content, isError := structured(t, serving(mcpServer{capture: refusing(t), captureStatus: refusingStatus(t)},
		toolCall("after_change", `{"paths":["app/code/Acme/Shop/etc/di.xml","app/code/Acme/Shop/view/frontend/templates/a.phtml","app/code/Acme/Shop/etc/db_schema.xml"]}`)))

	if isError {
		t.Fatalf("isError, content %v", content)
	}

	want := map[string]any{
		"cache_types":   []any{"block_html", "config", "full_page"},
		"di_compile":    true,
		"setup_upgrade": true,
		"static":        false,
		"commands":      []any{"bin/magento setup:upgrade", "bin/magento setup:di:compile", "bin/magento cache:clean block_html config full_page"},
	}

	for key, value := range want {
		if diff := cmp.Diff(value, content[key]); diff != "" {
			t.Errorf("%s (-want +got):\n%s", key, diff)
		}
	}

	if paths, _ := content["paths"].([]any); len(paths) != 3 {
		t.Errorf("paths = %v, want each path with its needs", content["paths"])
	}
}

func TestAfterChangeRefusesWhatItCannotRead(t *testing.T) {
	for _, arguments := range []string{`{}`, `{"paths":[]}`, `{"paths":"etc/di.xml"}`, `{"paths":[1]}`} {
		t.Run(arguments, func(t *testing.T) {
			quiet(t)

			if _, isError := answer(t, serving(mcpServer{capture: refusing(t), captureStatus: refusingStatus(t)},
				toolCall("after_change", arguments))); !isError {
				t.Fatal("isError = false, want it refused")
			}
		})
	}

	t.Run("applying without --write", func(t *testing.T) {
		quiet(t)

		text, isError := answer(t, serving(mcpServer{capture: refusing(t), captureStatus: refusingStatus(t)},
			toolCall("after_change", `{"paths":["etc/di.xml"],"apply":true}`)))

		if !isError || !strings.Contains(text, "--write") {
			t.Fatalf("= %q, %v; want a refusal naming --write", text, isError)
		}
	})
}

func TestAfterChangeAppliesWithWriteButNeverSetupUpgradeUnlessAsked(t *testing.T) {
	paths := `["app/code/Acme/Shop/etc/di.xml","app/code/Acme/Shop/view/frontend/layout/default.xml","app/code/Acme/Shop/etc/db_schema.xml"]`

	t.Run("not asked: caches and di:compile, and setup:upgrade reported as skipped", func(t *testing.T) {
		quiet(t)
		calls, server := looping(true, func([]string) (string, int) { return "done\n", 0 })

		content, isError := structured(t, serving(server, toolCall("after_change", `{"paths":`+paths+`,"apply":true}`)))

		want := [][]string{
			{"--no-json", "magento", "setup:di:compile"},
			{"--no-json", "magento", "cache:clean", "block_html", "config", "full_page", "layout"},
		}
		if diff := cmp.Diff(want, called(*calls)); diff != "" {
			t.Fatalf("runs (-want +got):\n%s", diff)
		}

		if isError || !cmp.Equal([]any{"bin/magento setup:upgrade"}, content["skipped"]) {
			t.Fatalf("content = %v, want setup:upgrade skipped", content)
		}
	})

	t.Run("asked: setup:upgrade first", func(t *testing.T) {
		quiet(t)
		calls, server := looping(true, func([]string) (string, int) { return "", 0 })

		serving(server, toolCall("after_change", `{"paths":`+paths+`,"apply":true,"setup_upgrade":true}`))

		if got := called(*calls); len(got) != 3 || !cmp.Equal([]string{"--no-json", "magento", "setup:upgrade"}, got[0]) {
			t.Fatalf("runs = %v, want setup:upgrade, di:compile, cache:clean", got)
		}
	})

	t.Run("a step that fails stops the rest and is an error", func(t *testing.T) {
		quiet(t)
		calls, server := looping(true, func([]string) (string, int) { return "Compilation failed\n", 1 })

		content, isError := structured(t, serving(server, toolCall("after_change", `{"paths":`+paths+`,"apply":true}`)))

		if len(*calls) != 1 || !isError {
			t.Fatalf("runs = %v, isError %v; want one run and an error", called(*calls), isError)
		}

		ran, _ := content["ran"].([]any)
		if len(ran) != 1 || ran[0].(map[string]any)["exit_code"] != float64(1) ||
			ran[0].(map[string]any)["output"] != "Compilation failed" {
			t.Fatalf("ran = %v, want the failing step with its output", content["ran"])
		}
	})

	t.Run("nothing needed runs nothing", func(t *testing.T) {
		quiet(t)

		_, isError := structured(t, serving(mcpServer{write: true, capture: refusing(t), captureStatus: refusingStatus(t)},
			toolCall("after_change", `{"paths":["app/code/Acme/Shop/Model/Price.php"],"apply":true}`)))

		if isError {
			t.Fatal("isError, want nothing to do to be no error")
		}
	})
}

func described(anonymised, at string) string {
	return `{"ok":true,"data":{"data":{"anonymised":"` + anonymised + `","anonymised_at":"` + at + `"},"paths":{"magento_dir":"."}}}`
}

const loggedExceptions = `[2026-11-01T09:00:00.000000+00:00] main.CRITICAL: customer jane@example.com could not pay
#0 /var/www/html/vendor/a.php(1): pay()
[2026-11-30T09:00:00.000000+00:00] main.CRITICAL: Price not found
#0 /var/www/html/app/code/Acme/Shop/Model/Price.php(12): find()
`

func TestMagentoExceptionsRefusesUnlessTheDatabaseIsAnonymised(t *testing.T) {
	for _, state := range []string{"unknown", "no", ""} {
		t.Run(state, func(t *testing.T) {
			quiet(t)

			calls, server := looping(false, func(args []string) (string, int) {
				if args[0] == "describe" {
					return described(state, ""), 0
				}

				t.Fatalf("ran %v, want the log never read", args)

				return "", 0
			})

			text, isError := answer(t, serving(server, toolCall("magento_exceptions", `{}`)))

			if !isError || !strings.Contains(text, "anonymised") || len(*calls) != 1 {
				t.Fatalf("= %q, %v after %v; want a refusal and only describe asked", text, isError, called(*calls))
			}
		})
	}
}

// TestMagentoExceptionsShowsOnlyWhatWasLoggedAfterTheAnonymisation: var/log is an ai-exclusion
// because it is about real customers, and what was logged before the database was anonymised was.
func TestMagentoExceptionsShowsOnlyWhatWasLoggedAfterTheAnonymisation(t *testing.T) {
	quiet(t)

	calls, server := looping(false, func(args []string) (string, int) {
		if args[0] == "describe" {
			return described("yes", "2026-11-15 10:00"), 0
		}

		return loggedExceptions, 0
	})

	text, isError := answer(t, serving(server, toolCall("magento_exceptions", `{"lines":30}`)))

	if isError || strings.Contains(text, "jane@example.com") || !strings.Contains(text, "main.CRITICAL: Price not found") ||
		!strings.Contains(text, "Price.php(12)") || !strings.Contains(text, "2 earlier lines") {
		t.Fatalf("= %q, %v; want only the entry logged after the anonymisation, and the rest counted", text, isError)
	}

	want := [][]string{{"describe", "--json"}, {"--no-json", "exec", "tail", "-n", "30", "var/log/exception.log"}}
	if diff := cmp.Diff(want, called(*calls)); diff != "" {
		t.Fatalf("runs (-want +got):\n%s", diff)
	}
}

func TestMagentoExceptionsReadsTheLogOfTheMagentoDirectory(t *testing.T) {
	quiet(t)

	calls, server := looping(false, func(args []string) (string, int) {
		if args[0] == "describe" {
			return strings.Replace(described("yes", "2026-11-15 10:00"), `"magento_dir":"."`, `"magento_dir":"src"`, 1), 0
		}

		return "", 1
	})

	text, isError := answer(t, serving(server, toolCall("magento_exceptions", `{"lines":5000}`)))

	if isError || !strings.Contains(text, "empty or not there") {
		t.Fatalf("= %q, %v; want a missing log said, not an error", text, isError)
	}

	if got := (*calls)[1].args; !cmp.Equal([]string{"--no-json", "exec", "tail", "-n", "200", "src/var/log/exception.log"}, got) {
		t.Fatalf("ran %v, want the capped tail of src/var/log/exception.log", got)
	}
}

func TestOnlyEntriesLoggedAfterAnInstantAreKept(t *testing.T) {
	since := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name, log, kept string
		dropped         int
	}{
		{"an entry with an offset", "[2026-11-15T10:59:59.000000+01:00] a\n[2026-11-15T11:00:01+00:00] b\n", "[2026-11-15T11:00:01+00:00] b", 1},
		{"Magento 2.3's form, read as UTC", "[2026-11-15 09:59:59] a\n[2026-11-15 10:00:00] b\n", "[2026-11-15 10:00:00] b", 1},
		{"lines before the first entry belong to an older one", "#3 trace\n[2026-11-16 00:00:00] b\n#0 x\n", "[2026-11-16 00:00:00] b\n#0 x", 1},
		{"nothing after", "[2026-01-01 00:00:00] a\n", "", 1},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			kept, dropped := exceptionsSince(tt.log, since)

			if kept != tt.kept || dropped != tt.dropped {
				t.Fatalf("= %q, %d; want %q, %d", kept, dropped, tt.kept, tt.dropped)
			}
		})
	}
}

// TestTheInstructionsNameTheToolsThatChangeTheEnvironment: generated from the catalogue, so a
// tool that becomes able to change something is named without anybody editing a sentence.
func TestTheInstructionsNameTheToolsThatChangeTheEnvironment(t *testing.T) {
	quiet(t)

	got := instructions(t, mcpServer{write: true, capture: refusing(t)})

	for _, want := range []string{"xdebug", "after_change", "cache_flush", "config_set"} {
		if !strings.Contains(got, want) {
			t.Errorf("instructions = %q, want %s named", got, want)
		}
	}

	for _, reads := range []string{"run_tests", "describe_project", "magento_exceptions", "environment_status"} {
		if strings.Contains(got, reads) {
			t.Errorf("instructions = %q, want %s left out: it only reads", got, reads)
		}
	}
}

func TestTheReadVariantsGiveWayToTheWriteOnes(t *testing.T) {
	quiet(t)

	for _, name := range []string{"xdebug", "after_change"} {
		read, write := tool(t, false, name), tool(t, true, name)

		readHints, _ := read["annotations"].(map[string]any)
		writeHints, _ := write["annotations"].(map[string]any)

		if readHints["readOnlyHint"] != true || writeHints["readOnlyHint"] != false {
			t.Errorf("%s: read %v, write %v; want read-only without --write only", name, readHints, writeHints)
		}
	}

	seen := map[string]int{}
	for _, name := range mcpToolNames(true) {
		seen[name]++
		if seen[name] > 1 {
			t.Errorf("%s offered twice with --write", name)
		}
	}
}
