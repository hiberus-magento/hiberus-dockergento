package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// verify runs whatever code checks the project has. Which checks and how they end is the use
// case's (app/verify_test.go); here, the options, the exit code and how the verdict is printed,
// which is verify.sh's text.
//

func verdict() core.Verification {
	return core.Verification{
		Project: "shop",
		Scope:   "everything",
		Failed:  1,
		Checks: []core.Check{
			{Name: "syntax", Status: "ok"},
			{Name: "static-analysis", Status: "failed", Problems: 2, Output: "a.php:1:wrong\nb.php:2:wrong"},
			{Name: "coding-standard", Status: "skipped", Output: "magento/magento-coding-standard is not installed"},
		},
	}
}

func TestVerifyRefusesAnUnknownOption(t *testing.T) {
	fake := answering(t)

	var stderr bytes.Buffer
	if code := verify([]string{"--changed", "--fast"}, io.Discard, &stderr, true); code != exitUsage {
		t.Fatalf("verify --fast = %d, want exitUsage", code)
	}

	var document struct {
		Error struct{ Type, Message, Hint string }
	}
	if err := json.Unmarshal(stderr.Bytes(), &document); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr.String())
	}

	if document.Error.Type != "invalid_argument" || document.Error.Message != "Unknown option: --fast" ||
		document.Error.Hint != "hm verify [--changed] [--all]" {
		t.Fatalf("error = %+v", document.Error)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("asked the engine %v, want nothing", fake.calls)
	}
}

func TestVerifyPassesItsOptionsOn(t *testing.T) {
	for args, want := range map[string]core.VerifyOptions{
		"":                {},
		"--changed":       {Changed: true},
		"--all":           {All: true},
		"--all --changed": {Changed: true, All: true},
	} {
		fake := answering(t)

		verify(strings.Fields(args), io.Discard, io.Discard, true)

		if len(fake.calls) != 1 || fake.calls[0].Method != "Verify" || fake.calls[0].VerifyOptions != want ||
			fake.calls[0].Dir != here() {
			t.Fatalf("verify %q asked %+v, want Verify %+v here", args, fake.calls, want)
		}
	}
}

func TestAVerificationAsJSON(t *testing.T) {
	fake := answering(t)
	fake.verification = verdict()

	var stdout bytes.Buffer
	if code := verify(nil, &stdout, io.Discard, true); code != exitError {
		t.Fatalf("verify with a failed check = %d, want exitError so it can be chained", code)
	}

	var document struct {
		Command string
		Data    struct {
			Scope  string
			Failed int
			Checks []core.Check
		}
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}

	if document.Command != "verify" || document.Data.Scope != "everything" || document.Data.Failed != 1 ||
		len(document.Data.Checks) != 3 || document.Data.Checks[1].Output != "a.php:1:wrong\nb.php:2:wrong" {
		t.Fatalf("document = %+v", document)
	}

	if bytes.Contains(stdout.Bytes(), []byte(`"project"`)) {
		t.Fatalf("the document carries the project, which verify.sh's did not: %s", stdout.String())
	}
}

func TestAPassingVerificationSucceeds(t *testing.T) {
	fake := answering(t)
	fake.verification = core.Verification{Project: "shop", Scope: "changed", Checks: []core.Check{{Name: "syntax", Status: "ok"}}}

	for _, jsonOutput := range []bool{true, false} {
		if code := verify(nil, io.Discard, io.Discard, jsonOutput); code != exitOK {
			t.Fatalf("verify (json %v) with nothing failed = %d", jsonOutput, code)
		}
	}
}

func TestAVerificationAsText(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.verification = verdict()

	var stdout bytes.Buffer
	if code := verify(nil, &stdout, io.Discard, false); code != exitError {
		t.Fatalf("verify = %d, want exitError", code)
	}

	want := "\nVerifying shop (everything)\n\n" +
		"  ✓  syntax            \n" +
		"  ✗  static-analysis    2 problem(s)\n" +
		"  –  coding-standard    magento/magento-coding-standard is not installed\n" +
		"\n" +
		"static-analysis\n" +
		"    a.php:1:wrong\n" +
		"    b.php:2:wrong\n" +
		"\n" +
		"\n"

	if stdout.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", stdout.String(), want)
	}
}

// TestAFailureShowsItsFirstThirtyLines: the rest is a scroll nobody reads; --json has all of it.
func TestAFailureShowsItsFirstThirtyLines(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")

	output := ""
	for line := 1; line <= 40; line++ {
		if line > 1 {
			output += "\n"
		}
		output += "line"
	}
	fake.verification = core.Verification{Project: "shop", Scope: "everything", Failed: 1,
		Checks: []core.Check{{Name: "syntax", Status: "failed", Problems: 40, Output: output}}}

	var stdout bytes.Buffer
	verify(nil, &stdout, io.Discard, false)

	if shown := bytes.Count(stdout.Bytes(), []byte("    line\n")); shown != 30 {
		t.Fatalf("showed %d lines of the failure, want 30", shown)
	}
}

// TestNothingToRunIsSaidAndIsNotAFailure: --changed with no PHP changed and no tools installed.
func TestNothingToRunIsSaidAndIsNotAFailure(t *testing.T) {
	fake := answering(t)
	t.Setenv("NO_COLOR", "1")
	fake.verification = core.Verification{Project: "shop", Scope: "changed", Checks: []core.Check{
		{Name: "syntax", Status: "skipped", Output: "no PHP files changed"},
	}}

	var stdout bytes.Buffer
	if code := verify([]string{"--changed"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("verify = %d, want exitOK", code)
	}

	want := "\nVerifying shop (changed)\n\n" +
		"  –  syntax             no PHP files changed\n" +
		"Nothing to run: this project has no verification tools installed.\n\n"

	if stdout.String() != want {
		t.Fatalf("stdout =\n%q\nwant\n%q", stdout.String(), want)
	}
}

func TestAVerifyRefusalKeepsItsCode(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.Refusal{Kind: "service_not_running", Code: exitService, Message: "Service 'phpfpm' is not running"}}}

	if code := verify(nil, io.Discard, io.Discard, true); code != exitService {
		t.Fatalf("verify without php = %d, want exitService", code)
	}
}

func TestTheRouterAnswersForVerify(t *testing.T) {
	fake := answering(t)

	Run([]string{"verify", "--all"}, io.Discard, io.Discard)

	if len(fake.calls) != 1 || fake.calls[0].Method != "Verify" {
		t.Fatalf("verify fell through to the shell implementation: %v", fake.calls)
	}
}
