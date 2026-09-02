package cli

import (
	"fmt"
	"io"
	"strings"
)

// shell opens a shell in the php container, which is `exec bash` and nothing else.
func shell(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	command := []string{"bash"}

	if len(args) > 0 && args[0] == "-r" {
		return execute(append([]string{"-r"}, command...), stdout, stderr, jsonOutput)
	}

	return execute(command, stdout, stderr, jsonOutput)
}

// masquerade replaces the personal data of this project's database with data that looks like it.
//
// It asks first, because it cannot be undone: the previous contents are gone and the way back is
// importing the dump again. `--yes` answers it, which is what a script or an agent passes.
func masquerade(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	for _, argument := range args {
		if strings.HasPrefix(argument, "-") {
			return failure(stderr, jsonOutput, "masquerade", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), binaryName()+" masquerade")
		}
	}

	if _, code := projectOr(stderr, jsonOutput, "masquerade"); code != 0 {
		return code
	}

	answer, err := ask("Are you sure you want to anonymise your database? [Y/n]:", "Y")
	if err != nil {
		return failure(stderr, jsonOutput, "masquerade", exitUsage, "input_required", err.Error(),
			binaryName()+" --yes masquerade")
	}

	//
	// Declining is not a failure. The shell implementation exits 1 here, but not on purpose: the
	// `if` is the last statement in the script and `set -e` turns a false condition into a
	// failing exit. A script asking whether it worked would be told no when the answer is that
	// somebody said no.
	//
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "", "y":
	default:
		return exitOK
	}

	return report(stderr, jsonOutput, "masquerade", engine(stdout, stderr, jsonOutput).Anonymise(here()))
}
