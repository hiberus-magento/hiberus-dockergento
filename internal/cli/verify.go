package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// verify runs whatever code checks the project has installed, and fails when one of them fails,
// so that it can be chained. A check whose tool is missing is skipped with the reason, never
// failed.
//
// Ported from verify.sh: the same options, checks, refusals, JSON document and text. One
// difference on purpose: a failure's output is printed one line per line, where the shell printed
// it as one line glued together with record separators — it folded the lines to store them and
// never unfolded them for printing.
func verify(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	options := core.VerifyOptions{}

	for _, argument := range args {
		switch argument {
		case "--all":
			options.All = true
		case "--changed":
			options.Changed = true
		default:
			return failure(stderr, jsonOutput, "verify", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" verify [--changed] [--all]")
		}
	}

	verification, err := newEngine(stdout, stderr, jsonOutput).Verify(here(), options)
	if err != nil {
		return report(stderr, jsonOutput, "verify", err)
	}

	outcome := exitOK
	if verification.Failed > 0 {
		outcome = exitError
	}

	if jsonOutput {
		if verification.Checks == nil {
			verification.Checks = []core.Check{}
		}

		if code := document(stdout, stderr, "verify", verification); code != exitOK {
			return code
		}

		return outcome
	}

	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, bold(fmt.Sprintf("Verifying %s (%s)\n\n", verification.Project, verification.Scope)))

	for _, check := range verification.Checks {
		switch check.Status {
		case core.CheckOK:
			fmt.Fprintf(stdout, "  %s✓%s  %-18s\n", code("\033[0;32m"), code(reset), check.Name)
		case core.CheckFailed:
			fmt.Fprintf(stdout, "  %s✗%s  %-18s %d problem(s)\n", code("\033[0;31m"), code(reset), check.Name, check.Problems)
		case core.CheckSkipped:
			fmt.Fprintf(stdout, "  %s–%s  %-18s %s\n", code("\033[1;37m"), code(reset), check.Name, check.Output)
		}
	}

	if verification.Failed > 0 {
		fmt.Fprint(stdout, "\n")

		for _, check := range verification.Checks {
			if check.Status != core.CheckFailed {
				continue
			}

			fmt.Fprint(stdout, warning(check.Name+"\n"))

			lines := strings.Split(check.Output, "\n")
			if len(lines) > 30 {
				lines = lines[:30]
			}

			for _, line := range lines {
				fmt.Fprintf(stdout, "    %s\n", line)
			}

			fmt.Fprint(stdout, "\n")
		}
	}

	if verification.Ran() == 0 {
		fmt.Fprint(stdout, good("Nothing to run: this project has no verification tools installed.\n\n"))

		return exitOK
	}

	fmt.Fprint(stdout, "\n")

	return outcome
}
