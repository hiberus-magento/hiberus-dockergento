package app

import (
	"bytes"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Verifier runs, in a project's php container, whatever code checks the project has installed.
//
// With agents writing code the bottleneck becomes checking it, and a check that depends on
// remembering four tools with four syntaxes does not happen. The tools are not the same in any two
// projects, so this looks at what is there and runs that: syntax always, since it needs nothing
// installed, and the rest when their binaries are in vendor/bin. Ported from verify.sh, its
// commands byte for byte, except that a changed file name bash would not read literally is quoted.
type Verifier struct {
	Engine  ports.ContainerEngine
	Runner  ports.ContainerRunner
	Changes ports.Changes

	// Progress is how a check that is a minute of a PHP process saying nothing says it started.
	// The label goes up before the process starts, which is the whole rule.
	Progress func(label string) func(ok bool, note string)

	// Binary is the name the tool was invoked as, for the sentence that says how to start php.
	Binary string
}

const verifiedService = "phpfpm"

// Verify runs the checks and reports each one. A failed check is a verdict, not an error: error is
// kept for when the checks could not be run at all.
func (v Verifier) Verify(project core.Project, options core.VerifyOptions) (core.Verification, error) {
	container, err := resolveContainer(v.Engine, project, verifiedService, v.Binary)
	if err != nil {
		return core.Verification{}, err
	}

	run := v.session(container)
	verification := core.Verification{Project: project.Name, Scope: "everything", Checks: []core.Check{}}

	//
	// --changed compares against where this branch left the main one, not against the last
	// commit: what matters when closing a task is everything the branch changed. Without a base,
	// everything is checked and that is said.
	//
	var files []string

	if options.Changed {
		base := ""

		for _, candidate := range []string{"main", "master"} {
			if v.Changes.Exists(project.Root, candidate) {
				base = candidate

				break
			}
		}

		if base != "" {
			files = v.Changes.ChangedSince(project.Root, base, "*.php")
			verification.Scope = "changed"
		} else {
			verification.Scope = "everything (no base branch to compare against)"
		}
	}

	record := func(check core.Check) { verification.Checks = append(verification.Checks, check) }

	syntax, err := v.syntax(run, verification.Scope == "changed", files)
	if err != nil {
		return core.Verification{}, err
	}

	record(syntax)

	// The tools look at the changed files when there are some, and at app/code otherwise — an
	// empty --changed included, as in the shell
	target := "app/code"
	if verification.Scope == "changed" && len(files) > 0 {
		target = shellWords(files)
	}

	tools := []struct{ name, binary, command, reason string }{
		{"coding-standard", "phpcs", "vendor/bin/phpcs --standard=Magento2 --extensions=php,phtml " + target,
			"magento/magento-coding-standard is not installed"},
		{"static-analysis", "phpstan", "vendor/bin/phpstan analyse --no-progress --error-format=raw " + target,
			"phpstan/phpstan is not installed"},
		{"formatting", "php-cs-fixer", "vendor/bin/php-cs-fixer fix --dry-run --diff " + target,
			"friendsofphp/php-cs-fixer is not installed"},
	}

	if options.All {
		tools = append(tools, struct{ name, binary, command, reason string }{
			"unit-tests", "phpunit", "vendor/bin/phpunit -c dev/tests/unit/phpunit.xml.dist", "phpunit/phpunit is not installed",
		})
	}

	for _, tool := range tools {
		check, err := v.withBinary(run, container, tool.name, tool.binary, tool.command, tool.reason)
		if err != nil {
			return core.Verification{}, err
		}

		record(check)
	}

	if options.All {
		output, status, err := run("bin/magento setup:di:compile")
		if err != nil {
			return core.Verification{}, err
		}

		if status == 0 {
			record(core.Check{Name: "di-compile", Status: core.CheckOK})
		} else {
			record(core.Check{Name: "di-compile", Status: core.CheckFailed, Problems: 1, Output: output})
		}
	} else {
		record(core.Check{Name: "unit-tests", Status: core.CheckSkipped, Output: "slow: run with --all"})
		record(core.Check{Name: "di-compile", Status: core.CheckSkipped, Output: "slow: run with --all"})
	}

	for _, check := range verification.Checks {
		if check.Status == core.CheckFailed {
			verification.Failed++
		}
	}

	return verification, nil
}

// session runs a shell command in the container with both streams together, the way verify.sh's
// `exec -T phpfpm bash -c "..." 2>&1` did, and hands back what it said without its trailing
// newlines, the way $(...) did.
func (v Verifier) session(container string) func(command string) (string, int, error) {
	return func(command string) (string, int, error) {
		var said bytes.Buffer

		status, err := v.Runner.Capture(container, []string{"bash", "-c", command}, &said, &said)

		return strings.TrimRight(said.String(), "\n"), status, err
	}
}

// syntax is the one check that needs nothing installed, and the one that catches what hurts most:
// a file that takes the whole site down.
func (v Verifier) syntax(run func(string) (string, int, error), changed bool, files []string) (core.Check, error) {
	done := v.announce("Checking syntax...")

	command := "find app/code app/design -name '*.php' -print0 2>/dev/null | xargs -0 -n1 -P4 php -l 2>&1 | grep -v '^No syntax errors' || true"

	if changed {
		if len(files) == 0 {
			done(true, "nothing to check")

			return core.Check{Name: "syntax", Status: core.CheckSkipped, Output: "no PHP files changed"}, nil
		}

		command = "for f in " + shellWords(files) +
			`; do [ -f "$f" ] && php -l "$f"; done | grep -v '^No syntax errors' || true`
	}

	// Its status says nothing: grep's, which the command already forgives
	output, _, err := run(command)
	if err != nil {
		done(false, "")

		return core.Check{}, err
	}

	done(true, "")

	if problems := countLines(output); problems > 0 {
		return core.Check{Name: "syntax", Status: core.CheckFailed, Problems: problems, Output: output}, nil
	}

	return core.Check{Name: "syntax", Status: core.CheckOK}, nil
}

// withBinary runs a tool when the project has it, and skips it with the reason when not.
func (v Verifier) withBinary(run func(string) (string, int, error), container, name, binary, command, reason string) (core.Check, error) {
	status, err := v.Runner.Capture(container, []string{"test", "-x", "vendor/bin/" + binary}, io.Discard, io.Discard)
	if err != nil || status != 0 {
		return core.Check{Name: name, Status: core.CheckSkipped, Output: reason}, nil //nolint:nilerr
	}

	done := v.announce("Running " + binary + "...")

	output, status, err := run(command)
	if err != nil {
		done(false, "")

		return core.Check{}, err
	}

	done(true, "")

	if status == 0 {
		return core.Check{Name: name, Status: core.CheckOK}, nil
	}

	return core.Check{Name: name, Status: core.CheckFailed, Problems: countLines(output), Output: output}, nil
}

func (v Verifier) announce(label string) func(bool, string) {
	if v.Progress == nil {
		return func(bool, string) {}
	}

	return v.Progress(label)
}

// shellWords joins file names for a bash command line, each one a single word: a name made only
// of characters bash reads literally goes as it is — what the shell port wrote — and any other is
// single-quoted, so a space or a metacharacter in a file name cannot split it or run something.
func shellWords(names []string) string {
	words := make([]string, len(names))

	for i, name := range names {
		words[i] = shellWord(name)
	}

	return strings.Join(words, " ")
}

func shellWord(name string) string {
	plain := name != ""

	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-+,:@%=", r)) {
			plain = false

			break
		}
	}

	if plain {
		return name
	}

	return "'" + strings.ReplaceAll(name, "'", `'\''`) + "'"
}

// countLines is the shell's `sed '/^$/d' | grep -c .`: the lines with anything on them.
func countLines(text string) int {
	count := 0

	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			count++
		}
	}

	return count
}
