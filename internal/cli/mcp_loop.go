package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The Magento loop: run the tests, look at xdebug, find out what a change needs and apply it, read
// the exceptions. What an agent does between one edit and the next, as typed tools instead of a
// shell. Each runs commands of this binary, like the rest; the rules that decide something are in
// core (core.PlanAfterChange, core.ParseJUnit), where they are tested on their own.
//

// mcpData is a tool's answer as data: the JSON as text, for the clients that read only that, and
// the same object as structuredContent.
func mcpData(value any, isError bool) jqValue {
	encoded, err := json.Marshal(value)
	if err != nil {
		return mcpContent(err.Error(), true)
	}

	data, _ := parseJQ(encoded)

	return mcpContent(data.pretty(), isError).with("structuredContent", data)
}

// statusOf is one command of this binary with its exit status, its output trimmed of the trailing
// newlines as a command substitution would.
func (s mcpServer) statusOf(combined bool, args ...string) (string, int) {
	output, status := s.captureStatus(s.dir, args, combined)

	return substituted(string(output)), status
}

// lastLines is the last n lines of text.
func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}

// refusedWithoutWrite is the answer to a read tool asked to change something: it does exist with
// --write, so the model is told how it would, not that it never could.
func refusedWithoutWrite(what string) jqValue {
	return mcpContent(what+" changes the environment, and this server was started without --write, "+
		"which a person decides in the MCP client's configuration ("+binaryName()+" mcp --config --write). "+
		"Ask them, or tell them the command to run.", true)
}

// testPath is a test file or directory as run_tests accepts one: relative, and nothing a shell
// would read as anything but a word, because `test-unit` puts it into a `sh -c` command line in
// the container.
var testPath = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_./-]*$`)

// testRun is run_tests' answer. The counts are there only when PHPUnit wrote its report: without
// one there is nothing to count, and a 0 would read as "no tests failed".
type testRun struct {
	Suite    string `json:"suite"`
	Passed   bool   `json:"passed"`
	ExitCode int    `json:"exit_code"`
	Source   string `json:"source"`
	*core.TestReport
	Tail string `json:"tail,omitempty"`
}

// runTests runs a suite with PHPUnit writing its JUnit report into the container's /tmp, reads it
// back through `exec` — which works whether the code is mounted or in a volume — and removes it.
func (s mcpServer) runTests(arguments jqValue) jqValue {
	suite := rawField(arguments, "suite", "unit")
	if suite != "unit" && suite != "integration" {
		return mcpContent("'"+suite+"' is not a suite. There are two: unit and integration.", true)
	}

	path := rawField(arguments, "path", "")
	if path != "" && (!testPath.MatchString(path) || strings.Contains("/"+path+"/", "/../")) {
		return mcpContent("'"+path+"' is not a path this tool runs: give a test file or directory relative to the "+
			"Magento root, made of letters, digits, '_', '-', '.' and '/', for example app/code/Acme/Shop/Test/Unit.", true)
	}

	junit := fmt.Sprintf("/tmp/hm-mcp-junit-%d.xml", time.Now().UnixNano())

	args := []string{"--no-json", "test-" + suite, "--log-junit", junit}
	if path != "" {
		args = append(args, path)
	}

	output, status := s.statusOf(true, args...)

	report, _ := s.statusOf(false, "--no-json", "exec", "cat", junit)
	s.statusOf(false, "--no-json", "exec", "rm", "-f", junit)

	run := testRun{Suite: suite, ExitCode: status}

	if parsed, err := core.ParseJUnit([]byte(report)); err == nil {
		run.Source, run.TestReport = "junit", &parsed
		run.Passed = status == 0 && parsed.Failures == 0 && parsed.Errors == 0

		// The report says which tests failed; when the run failed and it names none — a warning
		// turned into a failure, a crash after the last test — the output is the only answer
		if !run.Passed && len(parsed.Failed) == 0 {
			run.Tail = lastLines(output, 40)
		}
	} else {
		run.Source, run.Passed, run.Tail = "exit code", status == 0, lastLines(output, 40)
	}

	return mcpData(run, false)
}

// xdebug answers whether xdebug is on, from describe, or with --write turns it on or off.
func (s mcpServer) xdebug(arguments jqValue) jqValue {
	state := rawField(arguments, "state", "")

	switch state {
	case "":
		document, _ := parseJQ([]byte(s.run(false, "describe", "--json")))
		data, _ := document.field("data")
		tooling, _ := data.field("tooling")
		value, _ := tooling.field("xdebug")

		if value.kind != jqString {
			return mcpContent("Whether xdebug is on could not be read. The environment may not be running: try environment_status.", true)
		}

		return mcpData(map[string]string{"xdebug": value.text}, false)
	case "on", "off":
		if !s.write {
			return refusedWithoutWrite("Turning xdebug " + state)
		}

		output := s.run(true, "--no-json", "debug-"+state)
		if output == "" {
			output = "(done)"
		}

		return mcpContent(output, false)
	}

	return mcpContent("'"+state+"' is not a state: on or off, or leave it out to ask.", true)
}

// changeStep is one command after_change runs, and how it went.
type changeStep struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

// changeAnswer is after_change's answer: the plan, the commands that apply it, and, when it was
// applied, what ran and what was left for a person.
type changeAnswer struct {
	core.AfterChangePlan
	Commands []string     `json:"commands"`
	Ran      []changeStep `json:"ran,omitempty"`
	Skipped  []string     `json:"skipped,omitempty"`
}

// afterChange says what the changed files need (core.PlanAfterChange is the rule) and, with
// --write and apply, does it: setup:upgrade first, only when asked as well, because it changes the
// schema and the data; then di:compile; then the caches. A step that fails stops the rest.
func (s mcpServer) afterChange(arguments jqValue) jqValue {
	listed, _ := arguments.field("paths")

	var paths []string

	if listed.kind == jqArray {
		for _, item := range listed.items {
			if item.kind != jqString {
				paths = nil

				break
			}

			paths = append(paths, item.text)
		}
	}

	if len(paths) == 0 {
		return mcpContent(`Which files changed? Give them as paths, a list of strings relative to the project, for example ["app/code/Acme/Shop/etc/di.xml"].`, true)
	}

	apply, _ := arguments.field("apply")
	upgrade, _ := arguments.field("setup_upgrade")

	if (apply.kind == jqTrue || upgrade.kind == jqTrue) && !s.write {
		return refusedWithoutWrite("Applying what the change needs")
	}

	plan := core.PlanAfterChange(paths)

	type step struct {
		command string
		args    []string
	}

	var steps []step

	if plan.SetupUpgrade {
		steps = append(steps, step{"bin/magento setup:upgrade", []string{"setup:upgrade"}})
	}

	if plan.DiCompile {
		steps = append(steps, step{"bin/magento setup:di:compile", []string{"setup:di:compile"}})
	}

	if len(plan.CacheTypes) > 0 {
		steps = append(steps, step{"bin/magento cache:clean " + strings.Join(plan.CacheTypes, " "),
			append([]string{"cache:clean"}, plan.CacheTypes...)})
	}

	answer := changeAnswer{AfterChangePlan: plan, Commands: []string{}}
	for _, one := range steps {
		answer.Commands = append(answer.Commands, one.command)
	}

	if apply.kind != jqTrue {
		return mcpData(answer, false)
	}

	answer.Ran = []changeStep{}

	for _, one := range steps {
		if one.args[0] == "setup:upgrade" && upgrade.kind != jqTrue {
			answer.Skipped = append(answer.Skipped, one.command)

			continue
		}

		output, status := s.statusOf(true, append([]string{"--no-json", "magento"}, one.args...)...)
		answer.Ran = append(answer.Ran, changeStep{Command: one.command, ExitCode: status, Output: lastLines(output, 20)})

		if status != 0 {
			return mcpData(answer, true)
		}
	}

	return mcpData(answer, false)
}

// exceptionLog is the file magento_exceptions reads, relative to the Magento root.
const exceptionLog = "var/log/exception.log"

// magentoExceptions is the tail of exception.log, and only when the data in it cannot be about
// real customers: the database is anonymised, and when data/ai-exclusions.json marks the log as
// sensitive — it does, var/log — only what was logged after the anonymisation. The state is
// describe's, the same one the agent context and hm status report.
func (s mcpServer) magentoExceptions(arguments jqValue) jqValue {
	document, _ := parseJQ([]byte(s.run(false, "describe", "--json")))
	description, _ := document.field("data")
	data, _ := description.field("data")
	paths, _ := description.field("paths")

	anonymised := rawField(data, "anonymised", "")
	at := rawField(data, "anonymised_at", "")

	if anonymised != "yes" {
		return mcpContent("Refused: this environment's database is not known to be anonymised, and "+exceptionLog+
			" is about whatever the site did with that data — real customers' among it. Anonymise it with "+
			binaryName()+" masquerade, then ask again.", true)
	}

	sensitive, err := exceptionLogIsSensitive()
	if err != nil {
		return mcpContent("Refused: which files hold real customers' data could not be read ("+err.Error()+").", true)
	}

	var since time.Time

	if sensitive {
		recorded, err := time.ParseInLocation("2006-01-02 15:04", at, time.Local)
		if err != nil {
			return mcpContent("Refused: when the database was anonymised could not be read ('"+at+"').", true)
		}

		// The record has minutes, and an entry logged in that minute may predate the anonymisation
		since = recorded.Add(time.Minute)
	}

	lines := 50
	if asked, err := strconv.Atoi(rawField(arguments, "lines", "50")); err == nil && asked > 0 {
		lines = min(asked, 200)
	}

	file := exceptionLog
	if directory := rawField(paths, "magento_dir", ""); directory != "" && directory != "." {
		file = strings.TrimSuffix(directory, "/") + "/" + exceptionLog
	}

	output, _ := s.statusOf(false, "--no-json", "exec", "tail", "-n", strconv.Itoa(lines), file)
	if strings.TrimSpace(output) == "" {
		return mcpContent("("+file+" is empty or not there)", false)
	}

	if !sensitive {
		return mcpContent(output, false)
	}

	kept, dropped := exceptionsSince(output, since)

	note := ""
	if dropped > 0 {
		note = fmt.Sprintf("(%d earlier lines, logged before the database was anonymised on %s, are not shown)", dropped, at)
	}

	switch {
	case kept == "":
		return mcpContent("(nothing was logged since the database was anonymised on "+at+")", false)
	case note != "":
		return mcpContent(note+"\n"+kept, false)
	}

	return mcpContent(kept, false)
}

// exceptionLogIsSensitive says whether data/ai-exclusions.json marks exception.log, or a
// directory it is in, as holding real customers' data.
func exceptionLogIsSensitive() (bool, error) {
	contents, err := fs.ReadFile(toolAssets(), "data/ai-exclusions.json")
	if err != nil {
		return false, err
	}

	var file struct {
		Exclusions []struct {
			Path      string `json:"path"`
			Sensitive bool   `json:"sensitive"`
		} `json:"exclusions"`
	}

	if err := json.Unmarshal(contents, &file); err != nil {
		return false, err
	}

	for _, exclusion := range file.Exclusions {
		path := strings.TrimSuffix(exclusion.Path, "/")
		if exclusion.Sensitive && (path == exceptionLog || strings.HasPrefix(exceptionLog, path+"/")) {
			return true, nil
		}
	}

	return false, nil
}

// logEntry is the start of a Monolog entry: [2026-11-25T10:00:00.123456+00:00] in Magento 2.4,
// [2026-11-25 10:00:00] before it.
var logEntry = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}:\d{2})(\.\d+)?([+-]\d{2}:\d{2}|Z)?\]`)

// exceptionsSince keeps the entries of a log logged at or after since, each with the lines that
// follow it (a stack trace), and counts the lines dropped. Lines before the first entry belong to
// one that started earlier, which cannot be dated, so they are dropped. A time with no offset is
// read as UTC, which is what Magento logs in.
func exceptionsSince(log string, since time.Time) (string, int) {
	var kept []string

	keeping, dropped := false, 0

	for _, line := range strings.Split(strings.TrimRight(log, "\n"), "\n") {
		if found := logEntry.FindStringSubmatch(line); found != nil {
			zone := found[4]
			if zone == "" {
				zone = "Z"
			}

			logged, err := time.Parse(time.RFC3339, found[1]+"T"+found[2]+zone)
			keeping = err == nil && !logged.Before(since)
		}

		if keeping {
			kept = append(kept, line)
		} else {
			dropped++
		}
	}

	return strings.Join(kept, "\n"), dropped
}
