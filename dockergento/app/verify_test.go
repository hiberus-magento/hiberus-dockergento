package app

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// verify runs, in the php container, whatever checks the project has installed: syntax always,
// the coding standard, PHPStan and php-cs-fixer when their binaries are there, and the unit tests
// and DI compilation when asked for. Ported from verify.sh, its commands byte for byte.
//

const (
	syntaxEverywhere = "find app/code app/design -name '*.php' -print0 2>/dev/null | xargs -0 -n1 -P4 php -l 2>&1 | grep -v '^No syntax errors' || true"
	phpcsCommand     = "vendor/bin/phpcs --standard=Magento2 --extensions=php,phtml "
	phpstanCommand   = "vendor/bin/phpstan analyse --no-progress --error-format=raw "
	fixerCommand     = "vendor/bin/php-cs-fixer fix --dry-run --diff "
	phpunitCommand   = "vendor/bin/phpunit -c dev/tests/unit/phpunit.xml.dist"
	compileCommand   = "bin/magento setup:di:compile"
)

// answer is what one command run in the container says, on each stream, and its status.
type answer struct {
	out, complaint string
	status         int
}

// scripted is a container that answers each command from a script, and keeps the order they came
// in. A binary not in installed is not there; any command not scripted says nothing and succeeds.
type scripted struct {
	container string
	installed map[string]bool
	answers   map[string]answer
	ran       []string
}

func (s *scripted) Run(string, []string, []string, io.Writer) (int, error) { return 0, nil }

func (s *scripted) Feed(string, []string, io.Reader, io.Writer) (int, error) { return 0, nil }

func (s *scripted) Capture(container string, command []string, out, errors io.Writer) (int, error) {
	s.container = container

	if len(command) == 3 && command[0] == "test" && command[1] == "-x" {
		if s.installed[strings.TrimPrefix(command[2], "vendor/bin/")] {
			return 0, nil
		}

		return 1, nil
	}

	line := strings.Join(command[2:], " ")
	s.ran = append(s.ran, line)

	reply := s.answers[line]
	out.Write([]byte(reply.out))          //nolint:errcheck
	errors.Write([]byte(reply.complaint)) //nolint:errcheck

	return reply.status, nil
}

// branch is a repository with the bases it has and what it changed.
type branch struct {
	bases   map[string]bool
	changed []string
	since   string
}

func (b *branch) Exists(_, reference string) bool { return b.bases[reference] }

func (b *branch) ChangedSince(_, base, pathspec string) []string {
	b.since = base + " " + pathspec

	return b.changed
}

func verifier(container *scripted, repository *branch, labels *[]string) Verifier {
	return Verifier{
		Engine: &engine{containers: []core.Container{
			{ID: "php1", Project: "shop", ComposeService: "phpfpm", Running: true},
		}},
		Runner:  container,
		Changes: repository,
		Progress: func(label string) func(bool, string) {
			*labels = append(*labels, label)

			return func(_ bool, note string) {
				if note != "" {
					*labels = append(*labels, "("+note+")")
				}
			}
		},
		Binary: "hm",
	}
}

var verified = core.Project{Name: "shop", Root: "/code/shop"}

func TestVerifyNeedsThePHPContainerRunning(t *testing.T) {
	container := &scripted{}
	labels := []string{}
	v := verifier(container, &branch{}, &labels)
	v.Engine = &engine{containers: []core.Container{{ID: "php1", Project: "shop", ComposeService: "phpfpm"}}}

	_, err := v.Verify(verified, core.VerifyOptions{})

	if refusal := refusalOf(t, err); refusal.Kind != "service_not_running" || refusal.Code != 5 ||
		refusal.Message != "Service 'phpfpm' is not running" || refusal.Hint != "hm start phpfpm" {
		t.Fatalf("refusal = %+v", refusal)
	}

	if len(container.ran) != 0 || len(labels) != 0 {
		t.Fatalf("ran %v and announced %v, want nothing", container.ran, labels)
	}
}

// TestAProjectWithNothingStillHasItsSyntaxChecked: missing tools are skipped with the reason, and
// the slow ones wait for --all.
func TestAProjectWithNothingStillHasItsSyntaxChecked(t *testing.T) {
	container := &scripted{}
	labels := []string{}

	verification, err := verifier(container, &branch{}, &labels).Verify(verified, core.VerifyOptions{})
	if err != nil {
		t.Fatalf("Verify = %v", err)
	}

	want := core.Verification{
		Project: "shop",
		Scope:   "everything",
		Checks: []core.Check{
			{Name: "syntax", Status: "ok"},
			{Name: "coding-standard", Status: "skipped", Output: "magento/magento-coding-standard is not installed"},
			{Name: "static-analysis", Status: "skipped", Output: "phpstan/phpstan is not installed"},
			{Name: "formatting", Status: "skipped", Output: "friendsofphp/php-cs-fixer is not installed"},
			{Name: "unit-tests", Status: "skipped", Output: "slow: run with --all"},
			{Name: "di-compile", Status: "skipped", Output: "slow: run with --all"},
		},
	}

	if !reflect.DeepEqual(verification, want) {
		t.Fatalf("verification =\n%+v\nwant\n%+v", verification, want)
	}

	if !reflect.DeepEqual(container.ran, []string{syntaxEverywhere}) || container.container != "php1" {
		t.Fatalf("ran %v in %q, want only the syntax check in this project's php", container.ran, container.container)
	}

	if !reflect.DeepEqual(labels, []string{"Checking syntax..."}) {
		t.Fatalf("announced %v", labels)
	}
}

// TestAFileThatDoesNotParseFailsTheSyntax: one problem per line said, and what was said kept,
// what the container wrote on either stream.
func TestAFileThatDoesNotParseFailsTheSyntax(t *testing.T) {
	container := &scripted{answers: map[string]answer{
		syntaxEverywhere: {out: "PHP Parse error: in Broken.php on line 2\n", complaint: "Errors parsing Broken.php\n\n"},
	}}
	labels := []string{}

	verification, _ := verifier(container, &branch{}, &labels).Verify(verified, core.VerifyOptions{})

	syntax := verification.Checks[0]
	if syntax.Status != "failed" || syntax.Problems != 2 ||
		syntax.Output != "PHP Parse error: in Broken.php on line 2\nErrors parsing Broken.php" || verification.Failed != 1 {
		t.Fatalf("syntax = %+v, failed = %d", syntax, verification.Failed)
	}
}

func TestAnInstalledToolIsRunAndItsVerdictKept(t *testing.T) {
	container := &scripted{
		installed: map[string]bool{"phpcs": true, "phpstan": true},
		answers: map[string]answer{
			phpstanCommand + "app/code": {out: "a.php:1:wrong\nb.php:2:wrong\n", status: 1},
		},
	}
	labels := []string{}

	verification, _ := verifier(container, &branch{}, &labels).Verify(verified, core.VerifyOptions{})

	if got := verification.Checks[1]; got != (core.Check{Name: "coding-standard", Status: "ok"}) {
		t.Fatalf("coding-standard = %+v", got)
	}

	if got := verification.Checks[2]; got != (core.Check{Name: "static-analysis", Status: "failed", Problems: 2,
		Output: "a.php:1:wrong\nb.php:2:wrong"}) {
		t.Fatalf("static-analysis = %+v", got)
	}

	if verification.Failed != 1 {
		t.Fatalf("failed = %d, want 1", verification.Failed)
	}

	if want := []string{syntaxEverywhere, phpcsCommand + "app/code", phpstanCommand + "app/code"}; !reflect.DeepEqual(container.ran, want) {
		t.Fatalf("ran %v, want %v", container.ran, want)
	}

	if want := []string{"Checking syntax...", "Running phpcs...", "Running phpstan..."}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("announced %v", labels)
	}
}

// TestAToolThatFailsSilentlyHasNoProblemsCounted: the shell counted lines, and there are none.
func TestAToolThatFailsSilentlyHasNoProblemsCounted(t *testing.T) {
	container := &scripted{
		installed: map[string]bool{"php-cs-fixer": true},
		answers:   map[string]answer{fixerCommand + "app/code": {status: 8}},
	}
	labels := []string{}

	verification, _ := verifier(container, &branch{}, &labels).Verify(verified, core.VerifyOptions{})

	if got := verification.Checks[3]; got != (core.Check{Name: "formatting", Status: "failed"}) {
		t.Fatalf("formatting = %+v", got)
	}
}

func TestAllRunsTheUnitTestsAndTheCompilation(t *testing.T) {
	container := &scripted{
		installed: map[string]bool{"phpunit": true},
		answers:   map[string]answer{compileCommand: {out: "Fatal error\nmore\n", status: 255}},
	}
	labels := []string{}

	verification, _ := verifier(container, &branch{}, &labels).Verify(verified, core.VerifyOptions{All: true})

	if got := verification.Checks[4]; got != (core.Check{Name: "unit-tests", Status: "ok"}) {
		t.Fatalf("unit-tests = %+v", got)
	}

	// One problem, whatever it said: the shell did not count the compilation's lines
	if got := verification.Checks[5]; got != (core.Check{Name: "di-compile", Status: "failed", Problems: 1, Output: "Fatal error\nmore"}) {
		t.Fatalf("di-compile = %+v", got)
	}

	if want := []string{syntaxEverywhere, phpunitCommand, compileCommand}; !reflect.DeepEqual(container.ran, want) {
		t.Fatalf("ran %v, want %v", container.ran, want)
	}

	// The compilation has no label of its own, as in the shell
	if want := []string{"Checking syntax...", "Running phpunit..."}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("announced %v", labels)
	}
}

// TestChangedLooksOnlyAtWhatTheBranchChanged: against main, which is looked for before master.
func TestChangedLooksOnlyAtWhatTheBranchChanged(t *testing.T) {
	container := &scripted{installed: map[string]bool{"phpstan": true}}
	repository := &branch{bases: map[string]bool{"main": true, "master": true}, changed: []string{"app/code/A.php", "app/code/B.php"}}
	labels := []string{}

	verification, _ := verifier(container, repository, &labels).Verify(verified, core.VerifyOptions{Changed: true})

	if verification.Scope != "changed" || repository.since != "main *.php" {
		t.Fatalf("scope = %q, compared with %q", verification.Scope, repository.since)
	}

	want := []string{
		`for f in app/code/A.php app/code/B.php; do [ -f "$f" ] && php -l "$f"; done | grep -v '^No syntax errors' || true`,
		phpstanCommand + "app/code/A.php app/code/B.php",
	}
	if !reflect.DeepEqual(container.ran, want) {
		t.Fatalf("ran %v, want %v", container.ran, want)
	}
}

func TestMasterIsTheBaseWhenThereIsNoMain(t *testing.T) {
	repository := &branch{bases: map[string]bool{"master": true}}
	labels := []string{}

	verifier(&scripted{}, repository, &labels).Verify(verified, core.VerifyOptions{Changed: true}) //nolint:errcheck

	if repository.since != "master *.php" {
		t.Fatalf("compared with %q, want master", repository.since)
	}
}

// TestNoChangedFilesIsNothingToCheck: the syntax is skipped and said so, and the tools — as in the
// shell — fall back to app/code.
func TestNoChangedFilesIsNothingToCheck(t *testing.T) {
	container := &scripted{installed: map[string]bool{"phpcs": true}}
	repository := &branch{bases: map[string]bool{"main": true}}
	labels := []string{}

	verification, _ := verifier(container, repository, &labels).Verify(verified, core.VerifyOptions{Changed: true})

	if got := verification.Checks[0]; got != (core.Check{Name: "syntax", Status: "skipped", Output: "no PHP files changed"}) {
		t.Fatalf("syntax = %+v", got)
	}

	if want := []string{phpcsCommand + "app/code"}; !reflect.DeepEqual(container.ran, want) {
		t.Fatalf("ran %v, want %v", container.ran, want)
	}

	if want := []string{"Checking syntax...", "(nothing to check)", "Running phpcs..."}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("announced %v", labels)
	}
}

// TestWithoutABaseEverythingIsCheckedAndThatIsSaid: rather than checking a part while believing it
// is the whole.
func TestWithoutABaseEverythingIsCheckedAndThatIsSaid(t *testing.T) {
	container := &scripted{}
	labels := []string{}

	verification, _ := verifier(container, &branch{}, &labels).Verify(verified, core.VerifyOptions{Changed: true})

	if verification.Scope != "everything (no base branch to compare against)" {
		t.Fatalf("scope = %q", verification.Scope)
	}

	if !reflect.DeepEqual(container.ran, []string{syntaxEverywhere}) {
		t.Fatalf("ran %v", container.ran)
	}
}

// TestChangedFileNamesReachTheContainerShellAsOneWordEach: a name with a space or a shell
// metacharacter is one file, not two words or a second command. Plain names stay as they were.
func TestChangedFileNamesReachTheContainerShellAsOneWordEach(t *testing.T) {
	container := &scripted{installed: map[string]bool{"phpstan": true}}
	repository := &branch{bases: map[string]bool{"main": true},
		changed: []string{"app/code/My File.php", "app/code/a;touch x.php", "app/code/it's.php", "app/code/B.php"}}
	labels := []string{}

	verifier(container, repository, &labels).Verify(verified, core.VerifyOptions{Changed: true}) //nolint:errcheck

	files := `'app/code/My File.php' 'app/code/a;touch x.php' 'app/code/it'\''s.php' app/code/B.php`
	want := []string{
		`for f in ` + files + `; do [ -f "$f" ] && php -l "$f"; done | grep -v '^No syntax errors' || true`,
		phpstanCommand + files,
	}
	if !reflect.DeepEqual(container.ran, want) {
		t.Fatalf("ran %v, want %v", container.ran, want)
	}
}
